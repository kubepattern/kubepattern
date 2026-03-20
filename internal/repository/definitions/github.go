package definitions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Config struct {
	OrgName  string
	RepoName string
	Branch   string
	Token    string
}

type Client struct {
	config     Config
	httpClient *http.Client
}

type githubContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func NewClient(cfg Config) *Client {
	return &Client{
		config:     cfg,
		httpClient: &http.Client{},
	}
}

func (c *Client) ReadFile(filePath string) ([]byte, error) {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/definitions/%s",
		c.config.OrgName, c.config.RepoName, c.config.Branch, filePath)

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
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

func (c *Client) ReadAllDefinitions() (map[string][]byte, error) {
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
		if item.Type == "file" {
			content, err := c.ReadFile(item.Name)
			if err != nil {
				return nil, fmt.Errorf("error reading file %s: %w", item.Name, err)
			}
			filesData[item.Name] = content
		}
	}

	return filesData, nil
}
