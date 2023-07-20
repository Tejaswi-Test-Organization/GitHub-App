package main

import (
	"bytes"
	"context"
	"fmt"
	//"io/ioutil"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"time"
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
        	for {
			workflowContent, err := getWorkflowFileContent(client, "Tejaswi-Test-Organization", "Signature-Verification-Workflow", "workflows/gpg-signature-verification.yml")
			if err != nil {
				fmt.Println("Error: ", err)
			}
			bytesContent := []byte(workflowContent)
            		checkRepositoriesAndFileContents(context.Background(), client, "Tejaswi-Test-Organization", ".github/workflows/gpg-signature-verification.yml", bytesContent)
            		// Schedule the function to run every 30 minutes
            		time.Sleep(5 * time.Minute)
        	}
    	}()

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

func getWorkflowFileContent(client *github.Client, owner, repo, path string) (string, error) {
	fileContent, _, _, err := client.Repositories.GetContents(context.Background(), owner, repo, path, nil)
	if err != nil {
		return "", err
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", err
	}

	return content, nil
}

func checkRepositoriesAndFileContents(ctx context.Context, client *github.Client, orgName, fileName string, masterContents []byte) {
	// List all repositories in the organization
	repos, _, err := client.Repositories.ListByOrg(ctx, orgName, nil)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	for _, repo := range repos {
		// Check the contents of the particular file in each repository
		if repo.GetName() == "Signature-Verification-Workflow" || repo.GetName() == "GitHub-App" {
			fmt.Printf("Skipping repository %s\n", repo.GetName())
			continue
		}
		content, _, _, err := client.Repositories.GetContents(ctx, orgName, repo.GetName(), fileName, nil)
		if err != nil {
			fmt.Printf("Error checking file in %s: %v\n", repo.GetName(), err)
			continue
		}
		stringContent, er := content.GetContent()
		if er != nil {
			fmt.Println("Error: ", er)
		}
		contentBytes := []byte(stringContent)
		if bytes.Equal(contentBytes, masterContents) {
			fmt.Printf("Contents are identical for repository %s\n", repo.GetName())
		} else {
			fmt.Printf("Contents are NOT identical for repository %s\n", repo.GetName())
		}
	}
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
			ctx := context.Background()
			listOpts := &github.ListOptions{}
			commit, _, err := client.Repositories.GetCommit(ctx, "Tejaswi-Test-Organization", "Signature-Verification-Workflow", "main", listOpts)
			if err != nil {
				log.Fatalf("Error getting latest commit: %v", err)
			}
			fmt.Printf("Latest commit hash: %s\n", *commit.SHA)
		} else {
			fmt.Println("Workflow file does not exist in the repository.")
			fmt.Println("Retrieving Workflow file from Git repository.")
			workflowContent, err := getWorkflowFileContent(client, "Tejaswi-Test-Organization", "Signature-Verification-Workflow", "workflows/gpg-signature-verification.yml")
			if err != nil {
				log.Fatalf("Failed to retrieve YAML file from source repository: %v", err)
			}

			/*
				// Read the workflow file content from a separate file
				fmt.Println("Reading workflow file content from local directory.")
				workflowContent, err := ioutil.ReadFile("./gpg-signature-verification.yml")
				if err != nil {
					fmt.Printf("Failed to read workflow file: %v\n", err)
					return
				}
			*/
			workflowContentBytes := []byte(workflowContent)

			// Create the GitHub workflow file
			err = createGitHubWorkflowFile(client, repoOwner, repoName, workflowFilePath, workflowContentBytes)
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
