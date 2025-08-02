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
	client *github.Client
	config *Config
}

type Config struct {
	BaseURL    string
	Token      string
	Timeout    time.Duration
	MaxRetries int
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
	ID   int64
	Name string
	Slug string
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
		client: client,
		config: cfg,
	}, nil
}

func (c *Client) GetTeam(ctx context.Context, org, teamSlug string) (*Team, error) {
	team, _, err := c.client.Teams.GetTeamBySlug(ctx, org, teamSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return &Team{
		ID:   team.GetID(),
		Name: team.GetName(),
		Slug: team.GetSlug(),
	}, nil
}

func (c *Client) ListTeamRepositories(ctx context.Context, teamID int64) ([]*Repository, error) {
	opt := &github.ListOptions{PerPage: 100}
	var allRepos []*Repository

	for {
		repos, resp, err := c.client.Teams.ListTeamReposByID(ctx, teamID, teamID, opt)
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
	newPR := &github.NewPullRequest{
		Title: &pr.Title,
		Head:  &pr.Head,
		Base:  &pr.Base,
		Body:  &pr.Body,
	}

	pullRequest, _, err := c.client.PullRequests.Create(ctx, owner, repo, newPR)
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
	Query        string   // GitHub search query
	Owner        string   // Repository owner (user or organization)
	Language     string   // Programming language
	Stars        string   // Star count (e.g., ">100", "50..100")
	Size         string   // Repository size in KB
	Fork         bool     // Include forks
	Archived     bool     // Include archived repositories
	Sort         string   // Sort by: stars, forks, updated
	Order        string   // Sort order: asc, desc
	PerPage      int      // Results per page (max 100)
	MaxResults   int      // Maximum total results
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
		result, resp, err := c.client.Search.Repositories(ctx, query, searchOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to search repositories: %w", err)
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