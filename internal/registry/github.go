package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"kubepattern-go/internal/config"
)

// Config holds the GitHub Repository coordinates and authentication details
type Config struct {
	OrgName  string
	RepoName string
	Branch   string
	Token    string
}

// LoadConfig automatically parses environment variables and config into the Config struct.
func LoadConfig(appCfg *config.AppConfig) Config {
	return Config{
		OrgName:  fallback(appCfg.PatternRegistry.OrganizationName, "kubepattern"),
		RepoName: fallback(appCfg.PatternRegistry.RepositoryName, "registry"),
		Branch:   fallback(appCfg.PatternRegistry.RepositoryBranch, "main"),
		Token:    os.Getenv("GITHUB_TOKEN"),
	}
}

func fallback(val, def string) string {
	if val != "" {
		return val
	}
	return def
}

// Client wraps the configuration and the HTTP client used to interact with GitHub.
type Client struct {
	config     Config
	httpClient *http.Client
}

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

// ReadFile fetches the raw byte content of a specific file, respecting the context.
func (c *Client) ReadFile(ctx context.Context, filePath string) ([]byte, error) {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/definitions/%s",
		c.config.OrgName, c.config.RepoName, c.config.Branch, filePath)

	// Usa NewRequestWithContext per supportare i timeout!
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error during request creation: %w", err)
	}

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

// ReadAllDefinitions implements the Fetcher interface.
func (c *Client) ReadAllDefinitions(ctx context.Context) (map[string][]byte, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/definitions?ref=%s",
		c.config.OrgName, c.config.RepoName, c.config.Branch)

	// Usa NewRequestWithContext
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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
			content, err := c.ReadFile(ctx, item.Name)
			if err != nil {
				return nil, fmt.Errorf("error reading file %s: %w", item.Name, err)
			}
			filesData[item.Name] = content
		}
	}

	return filesData, nil
}
