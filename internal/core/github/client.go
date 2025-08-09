package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

type Client struct {
	client      *github.Client
	config      *Config
	rateLimiter *GitHubRateLimiter
}

type Config struct {
	BaseURL      string
	Token        string
	Timeout      time.Duration
	MaxRetries   int
	WaitForReset bool // Whether to wait when rate limited
}

type Repository struct {
	ID       int64
	Name     string
	FullName string
	CloneURL string
	SSHURL   string
	Private  bool
}

type Team struct {
	ID    int64
	OrgID int64
	Name  string
	Slug  string
}

type TreeEntry struct {
	Path string
	Type string
	Size int64
	SHA  string
}

type Tree struct {
	SHA     string
	Entries []TreeEntry
}

type CreateRepositoryOptions struct {
	Name         string
	Organization string
	Private      bool
	Description  string
}

type UpdateRepositoryOptions struct {
	DefaultBranch string
}

type CreateWebhookOptions struct {
	URL         string
	ContentType string
	Events      []string
	Active      bool
}

func NewClient(cfg *Config) (*Client, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.Token},
	)
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = cfg.Timeout

	var client *github.Client
	if cfg.BaseURL != "" && cfg.BaseURL != "https://api.github.com" {
		var err error
		client, err = github.NewClient(tc).WithEnterpriseURLs(cfg.BaseURL, cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub client with custom URL: %w", err)
		}
	} else {
		client = github.NewClient(tc)
	}

	return &Client{
		client:      client,
		config:      cfg,
		rateLimiter: NewGitHubRateLimiter(cfg.WaitForReset),
	}, nil
}

func (c *Client) GetTeam(ctx context.Context, org, teamSlug string) (*Team, error) {
	// Check rate limit before making request
	if err := c.rateLimiter.CheckRateLimit(ctx, false); err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	team, resp, err := c.client.Teams.GetTeamBySlug(ctx, org, teamSlug)

	// Update rate limit info from response
	c.rateLimiter.UpdateFromResponse(resp, false)

	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	// Extract organization ID from the team's organization
	orgID := int64(0)
	if team.Organization != nil {
		orgID = team.Organization.GetID()
	}

	return &Team{
		ID:    team.GetID(),
		OrgID: orgID,
		Name:  team.GetName(),
		Slug:  team.GetSlug(),
	}, nil
}

func (c *Client) ListTeamRepositories(ctx context.Context, team *Team) ([]*Repository, error) {
	opt := &github.ListOptions{PerPage: 100}
	var allRepos []*Repository

	for {
		// Check rate limit before making request
		if err := c.rateLimiter.CheckRateLimit(ctx, false); err != nil {
			return nil, fmt.Errorf("rate limit check failed: %w", err)
		}

		repos, resp, err := c.client.Teams.ListTeamReposByID(ctx, team.OrgID, team.ID, opt)

		// Update rate limit info from response
		c.rateLimiter.UpdateFromResponse(resp, false)

		if err != nil {
			return nil, fmt.Errorf("failed to list team repositories: %w", err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, &Repository{
				ID:       repo.GetID(),
				Name:     repo.GetName(),
				FullName: repo.GetFullName(),
				CloneURL: repo.GetCloneURL(),
				SSHURL:   repo.GetSSHURL(),
				Private:  repo.GetPrivate(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *Client) CreatePullRequest(ctx context.Context, owner, repo string, pr *PullRequestOptions) (*github.PullRequest, error) {
	// Check rate limit before making request
	if err := c.rateLimiter.CheckRateLimit(ctx, false); err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	newPR := &github.NewPullRequest{
		Title: &pr.Title,
		Head:  &pr.Head,
		Base:  &pr.Base,
		Body:  &pr.Body,
	}

	pullRequest, resp, err := c.client.PullRequests.Create(ctx, owner, repo, newPR)

	// Update rate limit info from response
	c.rateLimiter.UpdateFromResponse(resp, false)

	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	return pullRequest, nil
}

type PullRequestOptions struct {
	Title string
	Head  string
	Base  string
	Body  string
}

type SearchOptions struct {
	Query      string // GitHub search query
	Owner      string // Repository owner (user or organization)
	Language   string // Programming language
	Stars      string // Star count (e.g., ">100", "50..100")
	Size       string // Repository size in KB
	Fork       bool   // Include forks
	Archived   bool   // Include archived repositories
	Sort       string // Sort by: stars, forks, updated
	Order      string // Sort order: asc, desc
	PerPage    int    // Results per page (max 100)
	MaxResults int    // Maximum total results
}

func (c *Client) SearchRepositories(ctx context.Context, opts SearchOptions) ([]*Repository, error) {
	query := buildSearchQuery(opts)

	searchOpts := &github.SearchOptions{
		Sort:  opts.Sort,
		Order: opts.Order,
		ListOptions: github.ListOptions{
			PerPage: opts.PerPage,
		},
	}

	if searchOpts.ListOptions.PerPage == 0 {
		searchOpts.ListOptions.PerPage = 30
	}
	if searchOpts.ListOptions.PerPage > 100 {
		searchOpts.ListOptions.PerPage = 100
	}

	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 1000 // Default limit
	}

	var allRepos []*Repository

	for len(allRepos) < maxResults {
		// Check rate limit before making request
		if err := c.rateLimiter.CheckRateLimit(ctx, true); err != nil {
			return nil, fmt.Errorf("rate limit check failed: %w", err)
		}

		result, resp, err := c.client.Search.Repositories(ctx, query, searchOpts)

		// Update rate limit info from response
		c.rateLimiter.UpdateFromResponse(resp, true)

		if err != nil {
			// Check if it's a rate limit error
			if IsRateLimitError(err) && c.config.WaitForReset {
				// If configured to wait, retry after checking rate limit
				if err := c.rateLimiter.CheckRateLimit(ctx, true); err != nil {
					return nil, err
				}
				// Retry the request
				result, resp, err = c.client.Search.Repositories(ctx, query, searchOpts)
				c.rateLimiter.UpdateFromResponse(resp, true)
			}

			if err != nil {
				return nil, fmt.Errorf("failed to search repositories: %w", err)
			}
		}

		for _, repo := range result.Repositories {
			if len(allRepos) >= maxResults {
				break
			}

			allRepos = append(allRepos, &Repository{
				ID:       repo.GetID(),
				Name:     repo.GetName(),
				FullName: repo.GetFullName(),
				CloneURL: repo.GetCloneURL(),
				SSHURL:   repo.GetSSHURL(),
				Private:  repo.GetPrivate(),
			})
		}

		if resp.NextPage == 0 || len(allRepos) >= maxResults {
			break
		}
		searchOpts.Page = resp.NextPage
	}

	return allRepos, nil
}

func buildSearchQuery(opts SearchOptions) string {
	var parts []string

	if opts.Query != "" {
		parts = append(parts, opts.Query)
	}

	if opts.Owner != "" {
		parts = append(parts, fmt.Sprintf("user:%s", opts.Owner))
	}

	if opts.Language != "" {
		parts = append(parts, fmt.Sprintf("language:%s", opts.Language))
	}

	if opts.Stars != "" {
		parts = append(parts, fmt.Sprintf("stars:%s", opts.Stars))
	}

	if opts.Size != "" {
		parts = append(parts, fmt.Sprintf("size:%s", opts.Size))
	}

	if !opts.Fork {
		parts = append(parts, "fork:false")
	}

	if !opts.Archived {
		parts = append(parts, "archived:false")
	}

	if len(parts) == 0 {
		return "stars:>0" // Default query to get some results
	}

	return strings.Join(parts, " ")
}

func (c *Client) ValidateAccess(ctx context.Context, org, team string) error {
	user, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	fmt.Printf("Authenticated as: %s\n", user.GetLogin())

	if org != "" && team != "" {
		_, err = c.GetTeam(ctx, org, team)
		if err != nil {
			return fmt.Errorf("failed to access team %s in organization %s: %w", team, org, err)
		}
	}

	return nil
}

// ValidateTokenAccess validates basic API access without requiring org/team
func (c *Client) ValidateTokenAccess(ctx context.Context) error {
	user, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	fmt.Printf("Authenticated as: %s\n", user.GetLogin())
	return nil
}

// ListUserRepositories lists repositories accessible to the authenticated user
func (c *Client) ListUserRepositories(ctx context.Context) ([]*Repository, error) {
	var allRepos []*Repository

	opt := &github.RepositoryListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
		Sort:        "updated",
		Direction:   "desc",
	}

	for {
		repos, resp, err := c.client.Repositories.List(ctx, "", opt)
		if err != nil {
			return nil, fmt.Errorf("failed to list user repositories: %w", err)
		}

		for _, repo := range repos {
			if repo == nil {
				continue
			}

			r := &Repository{
				ID:       repo.GetID(),
				Name:     repo.GetName(),
				FullName: repo.GetFullName(),
				CloneURL: repo.GetCloneURL(),
				SSHURL:   repo.GetSSHURL(),
				Private:  repo.GetPrivate(),
			}
			allRepos = append(allRepos, r)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *Client) GetRepositoryTree(ctx context.Context, repoFullName, ref string, recursive bool) (*Tree, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository full name: %s", repoFullName)
	}
	owner, repo := parts[0], parts[1]

	tree, _, err := c.client.Git.GetTree(ctx, owner, repo, ref, recursive)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	result := &Tree{
		SHA:     tree.GetSHA(),
		Entries: make([]TreeEntry, len(tree.Entries)),
	}

	for i, entry := range tree.Entries {
		result.Entries[i] = TreeEntry{
			Path: entry.GetPath(),
			Type: entry.GetType(),
			Size: int64(entry.GetSize()),
			SHA:  entry.GetSHA(),
		}
	}

	return result, nil
}

func (c *Client) GetFileContent(ctx context.Context, repoFullName, filePath, ref string) (string, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repository full name: %s", repoFullName)
	}
	owner, repo := parts[0], parts[1]

	fileContent, _, _, err := c.client.Repositories.GetContents(ctx, owner, repo, filePath, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file content for %s: %w", filePath, err)
	}

	if fileContent == nil {
		return "", fmt.Errorf("file content is nil for %s", filePath)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content for %s: %w", filePath, err)
	}

	return content, nil
}

func (c *Client) GetRepository(ctx context.Context, repoFullName string) (*github.Repository, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository full name: %s", repoFullName)
	}
	owner, repo := parts[0], parts[1]

	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	return repository, nil
}

// GetRepositoryDefaultBranch returns the default branch name for a repository
func (c *Client) GetRepositoryDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	repository, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := repository.GetDefaultBranch()
	if defaultBranch == "" {
		return "main", nil // Fallback to main if empty
	}

	return defaultBranch, nil
}

// CreateRepository creates a new repository
func (c *Client) CreateRepository(ctx context.Context, opts *CreateRepositoryOptions) (*Repository, error) {
	repo := &github.Repository{
		Name:        github.String(opts.Name),
		Private:     github.Bool(opts.Private),
		Description: github.String(opts.Description),
		AutoInit:    github.Bool(false), // Don't create automatic README.md - we want an empty repository
	}

	var createdRepo *github.Repository
	var err error

	if opts.Organization != "" {
		createdRepo, _, err = c.client.Repositories.Create(ctx, opts.Organization, repo)
	} else {
		createdRepo, _, err = c.client.Repositories.Create(ctx, "", repo)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return &Repository{
		ID:       createdRepo.GetID(),
		Name:     createdRepo.GetName(),
		FullName: createdRepo.GetFullName(),
		CloneURL: createdRepo.GetCloneURL(),
		SSHURL:   createdRepo.GetSSHURL(),
		Private:  createdRepo.GetPrivate(),
	}, nil
}

// UpdateRepository updates repository settings
func (c *Client) UpdateRepository(ctx context.Context, owner, repo string, opts *UpdateRepositoryOptions) (*github.Repository, error) {
	update := &github.Repository{}

	if opts.DefaultBranch != "" {
		update.DefaultBranch = github.String(opts.DefaultBranch)
	}

	updatedRepo, _, err := c.client.Repositories.Edit(ctx, owner, repo, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}

	return updatedRepo, nil
}

// AddTeamToRepository adds a team to a repository with specific permissions
func (c *Client) AddTeamToRepository(ctx context.Context, org, teamSlug, repo, permission string) error {
	_, err := c.client.Teams.AddTeamRepoBySlug(ctx, org, teamSlug, org, repo, &github.TeamAddTeamRepoOptions{
		Permission: permission,
	})

	if err != nil {
		return fmt.Errorf("failed to add team %s to repository %s: %w", teamSlug, repo, err)
	}

	return nil
}

// CreateWebhook creates a webhook for a repository
func (c *Client) CreateWebhook(ctx context.Context, owner, repo string, opts *CreateWebhookOptions) (*github.Hook, error) {
	config := &github.HookConfig{
		URL:         github.String(opts.URL),
		ContentType: github.String(opts.ContentType),
	}

	hook := &github.Hook{
		Name:   github.String("web"),
		Config: config,
		Events: opts.Events,
		Active: github.Bool(opts.Active),
	}

	createdHook, _, err := c.client.Repositories.CreateHook(ctx, owner, repo, hook)
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return createdHook, nil
}

// GetRateLimitInfo returns current GitHub API rate limit information
func (c *Client) GetRateLimitInfo() RateLimitInfo {
	return c.rateLimiter.GetRateLimitInfo()
}

// GetLiveRateLimitInfo fetches current rate limit information from GitHub API
func (c *Client) GetLiveRateLimitInfo(ctx context.Context) (RateLimitInfo, error) {
	rateLimits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		// Return cached info if API call fails
		return c.rateLimiter.GetRateLimitInfo(), err
	}

	// Convert GitHub API response to our RateLimitInfo format
	info := RateLimitInfo{
		Core: RateInfo{
			Limit:     rateLimits.Core.Limit,
			Remaining: rateLimits.Core.Remaining,
			Reset:     rateLimits.Core.Reset.Time,
		},
		Search: RateInfo{
			Limit:     rateLimits.Search.Limit,
			Remaining: rateLimits.Search.Remaining,
			Reset:     rateLimits.Search.Reset.Time,
		},
	}

	// Update our internal rate limiter with fresh data
	c.rateLimiter.UpdateFromRateLimitResponse(rateLimits)

	return info, nil
}

// SetWaitForRateLimitReset configures whether to wait when rate limited
func (c *Client) SetWaitForRateLimitReset(wait bool) {
	c.rateLimiter.SetWaitForReset(wait)
}
