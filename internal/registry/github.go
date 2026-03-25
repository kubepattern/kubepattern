package definitions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Config holds the GitHub Repository coordinates and authentication details
// used to locate and access the target files.
type Config struct {
	OrgName  string
	RepoName string
	Branch   string
	Token    string
}

// LoadConfig automatically parses environment variables into the Config struct.
// It uses default values for the organization, repository, and branch if
// the corresponding environment variables are not set.
func LoadConfig() Config {
	cfg := Config{
		OrgName:  getEnvOrDefault("GITHUB_ORG", "kubepattern"),
		RepoName: getEnvOrDefault("GITHUB_REPO", "registry"),
		Branch:   getEnvOrDefault("GITHUB_BRANCH", "main"),
		Token:    os.Getenv("GITHUB_TOKEN"), // No default; required for private repos
	}

	return cfg
}

// getEnvOrDefault is a helper function that returns the environment variable
// if it exists and is not empty; otherwise, it returns the fallback value.
func getEnvOrDefault(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

// Client wraps the configuration and the HTTP client used to interact with
// the GitHub API and raw content servers.
type Client struct {
	config     Config
	httpClient *http.Client
}

// githubContent represents the minimal structure of a GitHub API response
// when listing directory contents.
type githubContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// NewClient initializes and returns a new GitHub definition client.
func NewClient(cfg Config) *Client {
	return &Client{
		config:     cfg,
		httpClient: &http.Client{},
	}
}

// ReadFile fetches the raw byte content of a specific file located within
// the "definitions" directory of the configured repository.
func (c *Client) ReadFile(filePath string) ([]byte, error) {
	// Construct the URL for the raw file content
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/definitions/%s",
		c.config.OrgName, c.config.RepoName, c.config.Branch, filePath)

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error during request creation: %w", err)
	}

	// Apply authentication headers if a token is provided
	if c.config.Token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.Token))
		req.Header.Add("Accept", "application/vnd.github.v3.raw")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error during request execution: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned status code %d for file %s", resp.StatusCode, filePath)
	}

	return io.ReadAll(resp.Body)
}

// ReadAllDefinitions lists all files in the "definitions" directory via the GitHub API
// and then performs individual fetches to retrieve their full content.
// It returns a map where the key is the filename and the value is the file content.
func (c *Client) ReadAllDefinitions() (map[string][]byte, error) {
	// API URL to list the contents of the 'definitions' folder
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/definitions?ref=%s",
		c.config.OrgName, c.config.RepoName, c.config.Branch)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error during API request creation: %w", err)
	}

	if c.config.Token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.config.Token))
		req.Header.Add("Accept", "application/vnd.github.v3+json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error during API request execution: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error from GitHub API: %s (status code: %d)", resp.Status, resp.StatusCode)
	}

	var contents []githubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, fmt.Errorf("error decoding GitHub API response: %w", err)
	}

	filesData := make(map[string][]byte)
	for _, item := range contents {
		isYaml := strings.HasSuffix(item.Name, ".yaml") || strings.HasSuffix(item.Name, ".yml")

		if item.Type == "file" && isYaml {
			content, err := c.ReadFile(item.Name)
			if err != nil {
				return nil, fmt.Errorf("error reading file %s: %w", item.Name, err)
			}
			filesData[item.Name] = content
		}
	}

	return filesData, nil
}
