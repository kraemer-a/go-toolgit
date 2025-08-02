package bitbucket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	config     *Config
	baseURL    string
}

type Config struct {
	BaseURL    string // Bitbucket Server base URL (e.g., https://bitbucket.company.com)
	Username   string // Username for basic auth
	Password   string // Password or personal access token
	Timeout    time.Duration
	MaxRetries int
}

type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string // project/repo format
	CloneURL string
	SSHURL   string
	Private  bool `json:"public"` // Note: Bitbucket uses "public" field (inverted)
	Project  string
}

type Project struct {
	Key         string `json:"key"`
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
}

type PullRequest struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
	Links       Links  `json:"links"`
}

type PullRequestOptions struct {
	Title       string
	Description string
	FromRef     Ref
	ToRef       Ref
}

type Ref struct {
	ID         string     `json:"id"`
	Repository Repository `json:"repository"`
}

type Links struct {
	Self []Link `json:"self"`
}

type Link struct {
	Href string `json:"href"`
}

// Response structures for Bitbucket REST API
type RepositoriesResponse struct {
	Values        []BitbucketRepository `json:"values"`
	Size          int                   `json:"size"`
	Limit         int                   `json:"limit"`
	IsLastPage    bool                  `json:"isLastPage"`
	Start         int                   `json:"start"`
	NextPageStart int                   `json:"nextPageStart,omitempty"`
}

type BitbucketRepository struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name"`
	ScmID   string          `json:"scmId"`
	State   string          `json:"state"`
	Public  bool            `json:"public"`
	Project BitbucketProject `json:"project"`
	Links   RepositoryLinks `json:"links"`
}

type BitbucketProject struct {
	Key    string `json:"key"`
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Public bool   `json:"public"`
}

type RepositoryLinks struct {
	Clone []CloneLink `json:"clone"`
	Self  []Link      `json:"self"`
}

type CloneLink struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

func NewClient(cfg *Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("baseURL is required for Bitbucket Server")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("username is required for Bitbucket Server")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password/token is required for Bitbucket Server")
	}

	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	// Ensure baseURL ends with /rest/api/1.0 for Bitbucket Server REST API
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/rest/api/1.0") {
		baseURL += "/rest/api/1.0"
	}

	return &Client{
		httpClient: httpClient,
		config:     cfg,
		baseURL:    baseURL,
	}, nil
}

func (c *Client) ValidateAccess(ctx context.Context) error {
	// Test access by getting user info
	req, err := c.newRequest(ctx, "GET", "/application-properties", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate access: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("access validation failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) ListProjectRepositories(ctx context.Context, projectKey string) ([]*Repository, error) {
	var allRepos []*Repository
	start := 0
	limit := 50

	for {
		endpoint := fmt.Sprintf("/projects/%s/repos", projectKey)
		params := url.Values{
			"start": {strconv.Itoa(start)},
			"limit": {strconv.Itoa(limit)},
		}

		req, err := c.newRequest(ctx, "GET", endpoint+"?"+params.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to list repositories, status: %d", resp.StatusCode)
		}

		var reposResp RepositoriesResponse
		if err := json.NewDecoder(resp.Body).Decode(&reposResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// Convert Bitbucket repositories to our format
		for _, bbRepo := range reposResp.Values {
			repo := &Repository{
				ID:       bbRepo.ID,
				Name:     bbRepo.Name,
				FullName: fmt.Sprintf("%s/%s", bbRepo.Project.Key, bbRepo.Name),
				Private:  !bbRepo.Public,
				Project:  bbRepo.Project.Key,
			}

			// Extract clone URLs
			for _, link := range bbRepo.Links.Clone {
				if link.Name == "http" || link.Name == "https" {
					repo.CloneURL = link.Href
				} else if link.Name == "ssh" {
					repo.SSHURL = link.Href
				}
			}

			allRepos = append(allRepos, repo)
		}

		if reposResp.IsLastPage {
			break
		}
		start = reposResp.NextPageStart
	}

	return allRepos, nil
}

func (c *Client) CreatePullRequest(ctx context.Context, projectKey, repoSlug string, pr *PullRequestOptions) (*PullRequest, error) {
	endpoint := fmt.Sprintf("/projects/%s/repos/%s/pull-requests", projectKey, repoSlug)

	reqBody := map[string]interface{}{
		"title":       pr.Title,
		"description": pr.Description,
		"fromRef": map[string]interface{}{
			"id": pr.FromRef.ID,
			"repository": map[string]interface{}{
				"slug": repoSlug,
				"project": map[string]interface{}{
					"key": projectKey,
				},
			},
		},
		"toRef": map[string]interface{}{
			"id": pr.ToRef.ID,
			"repository": map[string]interface{}{
				"slug": repoSlug,
				"project": map[string]interface{}{
					"key": projectKey,
				},
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := c.newRequest(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create pull request, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var pullRequest PullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pullRequest); err != nil {
		return nil, fmt.Errorf("failed to decode pull request response: %w", err)
	}

	return &pullRequest, nil
}

func (c *Client) GetRepository(ctx context.Context, projectKey, repoSlug string) (*Repository, error) {
	endpoint := fmt.Sprintf("/projects/%s/repos/%s", projectKey, repoSlug)

	req, err := c.newRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repository, status: %d", resp.StatusCode)
	}

	var bbRepo BitbucketRepository
	if err := json.NewDecoder(resp.Body).Decode(&bbRepo); err != nil {
		return nil, fmt.Errorf("failed to decode repository response: %w", err)
	}

	repo := &Repository{
		ID:       bbRepo.ID,
		Name:     bbRepo.Name,
		FullName: fmt.Sprintf("%s/%s", bbRepo.Project.Key, bbRepo.Name),
		Private:  !bbRepo.Public,
		Project:  bbRepo.Project.Key,
	}

	// Extract clone URLs
	for _, link := range bbRepo.Links.Clone {
		if link.Name == "http" || link.Name == "https" {
			repo.CloneURL = link.Href
		} else if link.Name == "ssh" {
			repo.SSHURL = link.Href
		}
	}

	return repo, nil
}

func (c *Client) newRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Request, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	// Set basic auth
	req.SetBasicAuth(c.config.Username, c.config.Password)

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

func (pr *PullRequest) GetHTMLURL() string {
	// Construct URL from self links
	if len(pr.Links.Self) > 0 {
		return pr.Links.Self[0].Href
	}
	return ""
}