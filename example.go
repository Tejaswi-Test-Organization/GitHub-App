package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
	"golang.org/x/oauth2"
	//"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v39/github"
)

func main() {
	/*
		appID := int64(359752)
		privateKeyPath := "/Users/tej/Downloads/workflowtester.2023-07-14.private-key.pem"

		itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, privateKeyPath)
		if err != nil {
			log.Fatalf("Failed to create Apps transport: %v", err)
		}

		client := github.NewClient(&http.Client{Transport: itr})
	*/

	// client := github.NewClient(http.DefaultClient)

	token := "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"
	tokenClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))

	client := github.NewClient(tokenClient)

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, []byte(""))
		if err != nil {
			log.Printf("Failed to validate payload: %v", err)
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			log.Printf("Failed to parse webhook: %v", err)
			http.Error(w, "Failed to parse webhook", http.StatusBadRequest)
			return
		}

		switch event := event.(type) {
		case *github.PullRequestEvent:
			handlePullRequestEvent(event, client)
		case *github.RepositoryEvent:
			handleRepositoryEvent(event, client)
		default:
			log.Printf("Unhandled event type: %T", event)
		}

		w.WriteHeader(http.StatusOK)
	})

	addr := ":41445"
	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("GoLang Github Application listening on %s", addr)

	// Wait for a signal to stop the server
	<-context.TODO().Done()

	// Gracefully shutdown the server
	if err := server.Shutdown(context.TODO()); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}
}

func handlePullRequestEvent(event *github.PullRequestEvent, client *github.Client) {
	fmt.Printf("Calling handlePullRequestEvent().")
	action := event.GetAction()
	pr := event.GetPullRequest()

	log.Printf("Pull request %s: %s", action, pr.GetHTMLURL())
}

func handleRepositoryEvent(event *github.RepositoryEvent, client *github.Client) {
	if event.GetAction() == "created" {
		fmt.Println("Event received for repository creation.")
		// Get the necessary information from the event
		repo := event.GetRepo()
		repoOwner := repo.GetOwner().GetLogin()
		repoName := repo.GetName()

		fmt.Printf("Repo Owner: %s\n", repoOwner)
		fmt.Printf("Repo Name: %s\n", repoName)

		// Check if the workflow file already exists
		workflowFilePath := ".github/workflows/gpg-signature-verification.yml"
		exists, err := checkFileExists(client, repoOwner, repoName, workflowFilePath)
		if err != nil {
			fmt.Printf("Failed to check file existence: %v\n", err)
			return
		}

		if exists {
			fmt.Printf("Workflow file already exists in repository %s/%s\n", repoOwner, repoName)
		} else {
			fmt.Println("Workflow file does not exist in the repository.")
			// Read the workflow file content from a separate file
			fmt.Println("Reading workflow file content from local directory.")
			workflowContent, err := ioutil.ReadFile("./gpg-signature-verification.yml")
			if err != nil {
				fmt.Printf("Failed to read workflow file: %v\n", err)
				return
			}

			// Create the GitHub workflow file
			err = createGitHubWorkflowFile(client, repoOwner, repoName, workflowFilePath, workflowContent)
			if err != nil {
				fmt.Printf("Failed to create GitHub workflow file: %v\n", err)
			} else {
				fmt.Printf("GitHub workflow file created successfully for repository %s/%s\n", repoOwner, repoName)
			}
		}
	}
}

func checkFileExists(client *github.Client, repoOwner, repoName, filePath string) (bool, error) {
	_, _, _, err := client.Repositories.GetContents(context.Background(), repoOwner, repoName, filePath, &github.RepositoryContentGetOptions{})
	if err != nil {
		if _, ok := err.(*github.ErrorResponse); ok && err.(*github.ErrorResponse).Response.StatusCode == http.StatusNotFound {
			return false, nil // File doesn't exist
		}
		fmt.Println("Error:", err)
		return false, err //some other error occurred
	}
	return true, nil
}

func createGitHubWorkflowFile(client *github.Client, repoOwner, repoName string, filePath string, content []byte) error {
	fmt.Println("Creating workflow file at git repository using GitHub API.")
	commitMessage := "Add GitHub workflow file for GPG signature verification"

	// Create the file using the GitHub API
	_, _, err := client.Repositories.CreateFile(context.Background(), repoOwner, repoName, filePath, &github.RepositoryContentFileOptions{
		Message: github.String(commitMessage),
		Content: content,
	})
	return err
}
