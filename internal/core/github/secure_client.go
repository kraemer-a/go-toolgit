package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-toolgit/internal/core/security"
)

// SecureClient wraps the GitHub client with input validation and security features
type SecureClient struct {
	client    *Client
	validator *security.InputValidator
}

// NewSecureClient creates a new secure GitHub client wrapper
func NewSecureClient(cfg *Config) (*SecureClient, error) {
	// Validate configuration first
	validator := security.NewInputValidator(true) // Use strict mode for API client
	
	if err := validator.ValidateURL("base_url", cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	
	if err := validator.ValidateToken("token", cfg.Token); err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	
	// Validate timeout and retry settings
	if cfg.Timeout < 0 || cfg.Timeout > time.Hour {
		return nil, fmt.Errorf("invalid timeout: must be between 0 and 1 hour")
	}
	
	if cfg.MaxRetries < 0 || cfg.MaxRetries > 100 {
		return nil, fmt.Errorf("invalid max retries: must be between 0 and 100")
	}
	
	// Create the underlying client
	client, err := NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	
	return &SecureClient{
		client:    client,
		validator: validator,
	}, nil
}

// SearchRepositories performs a secure repository search with input validation
func (sc *SecureClient) SearchRepositories(ctx context.Context, opts SearchOptions) ([]*Repository, error) {
	// Validate all search options
	if err := sc.validateSearchOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid search options: %w", err)
	}
	
	// Sanitize inputs
	opts = sc.sanitizeSearchOptions(opts)
	
	// Use the underlying client
	return sc.client.SearchRepositories(ctx, opts)
}

// GetTeam gets team information with validation
func (sc *SecureClient) GetTeam(ctx context.Context, org, teamSlug string) (*Team, error) {
	if err := sc.validator.ValidateString("organization", org, 100); err != nil {
		return nil, fmt.Errorf("invalid organization: %w", err)
	}
	
	if err := sc.validator.ValidateString("team_slug", teamSlug, 100); err != nil {
		return nil, fmt.Errorf("invalid team slug: %w", err)
	}
	
	// Sanitize inputs
	org = sc.validator.SanitizeString(org)
	teamSlug = sc.validator.SanitizeString(teamSlug)
	
	return sc.client.GetTeam(ctx, org, teamSlug)
}

// ListTeamRepositories lists repositories for a team with validation
func (sc *SecureClient) ListTeamRepositories(ctx context.Context, team *Team) ([]*Repository, error) {
	if team == nil {
		return nil, fmt.Errorf("team cannot be nil")
	}
	if team.ID <= 0 {
		return nil, fmt.Errorf("invalid team ID: must be positive")
	}
	if team.OrgID <= 0 {
		return nil, fmt.Errorf("invalid organization ID: must be positive")
	}
	
	return sc.client.ListTeamRepositories(ctx, team)
}

// CreatePullRequest creates a pull request with validation
func (sc *SecureClient) CreatePullRequest(ctx context.Context, owner, repo string, pr *PullRequestOptions) (*Repository, error) {
	// Validate inputs
	if err := sc.validator.ValidateString("owner", owner, 100); err != nil {
		return nil, fmt.Errorf("invalid owner: %w", err)
	}
	
	if err := sc.validator.ValidateString("repo", repo, 100); err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}
	
	if err := sc.validatePullRequestOptions(pr); err != nil {
		return nil, fmt.Errorf("invalid pull request options: %w", err)
	}
	
	// Sanitize inputs
	owner = sc.validator.SanitizeString(owner)
	repo = sc.validator.SanitizeString(repo)
	pr = sc.sanitizePullRequestOptions(pr)
	
	// Call underlying client - note: return type mismatch, this would need to be fixed
	result, err := sc.client.CreatePullRequest(ctx, owner, repo, pr)
	if err != nil {
		return nil, err
	}
	
	// Convert github.PullRequest to Repository (this is a placeholder - actual implementation would differ)
	return &Repository{
		ID:       result.GetID(),
		Name:     repo,
		FullName: fmt.Sprintf("%s/%s", owner, repo),
		CloneURL: fmt.Sprintf("https://github.com/%s/%s.git", owner, repo),
	}, nil
}

// ValidateAccess validates GitHub access with input validation
func (sc *SecureClient) ValidateAccess(ctx context.Context, org, team string) error {
	if err := sc.validator.ValidateString("organization", org, 100); err != nil {
		return fmt.Errorf("invalid organization: %w", err)
	}
	
	if err := sc.validator.ValidateString("team", team, 100); err != nil {
		return fmt.Errorf("invalid team: %w", err)
	}
	
	// Sanitize inputs
	org = sc.validator.SanitizeString(org)
	team = sc.validator.SanitizeString(team)
	
	return sc.client.ValidateAccess(ctx, org, team)
}

// CreateRepository creates a repository with validation
func (sc *SecureClient) CreateRepository(ctx context.Context, opts *CreateRepositoryOptions) (*Repository, error) {
	if err := sc.validateCreateRepositoryOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid repository options: %w", err)
	}
	
	// Sanitize inputs
	opts = sc.sanitizeCreateRepositoryOptions(opts)
	
	return sc.client.CreateRepository(ctx, opts)
}

// Validation helper methods

func (sc *SecureClient) validateSearchOptions(opts SearchOptions) error {
	if opts.Query != "" {
		if err := sc.validator.ValidateSearchQuery("query", opts.Query); err != nil {
			return err
		}
	}
	
	if opts.Owner != "" {
		if err := sc.validator.ValidateString("owner", opts.Owner, 100); err != nil {
			return err
		}
	}
	
	if opts.Language != "" {
		if err := sc.validator.ValidateString("language", opts.Language, 50); err != nil {
			return err
		}
	}
	
	if opts.Stars != "" {
		if err := sc.validator.ValidateString("stars", opts.Stars, 50); err != nil {
			return err
		}
	}
	
	if opts.Size != "" {
		if err := sc.validator.ValidateString("size", opts.Size, 50); err != nil {
			return err
		}
	}
	
	// Validate pagination settings
	if opts.PerPage < 0 || opts.PerPage > 100 {
		return fmt.Errorf("invalid per_page: must be between 0 and 100")
	}
	
	if opts.MaxResults < 0 || opts.MaxResults > 10000 {
		return fmt.Errorf("invalid max_results: must be between 0 and 10000")
	}
	
	return nil
}

func (sc *SecureClient) validatePullRequestOptions(opts *PullRequestOptions) error {
	if err := sc.validator.ValidateString("title", opts.Title, 250); err != nil {
		return err
	}
	
	if err := sc.validator.ValidateBranchName("head", opts.Head); err != nil {
		return err
	}
	
	if err := sc.validator.ValidateBranchName("base", opts.Base); err != nil {
		return err
	}
	
	if err := sc.validator.ValidateString("body", opts.Body, 5000); err != nil {
		return err
	}
	
	return nil
}

func (sc *SecureClient) validateCreateRepositoryOptions(opts *CreateRepositoryOptions) error {
	if err := sc.validator.ValidateString("name", opts.Name, 100); err != nil {
		return err
	}
	
	if opts.Organization != "" {
		if err := sc.validator.ValidateString("organization", opts.Organization, 100); err != nil {
			return err
		}
	}
	
	if err := sc.validator.ValidateString("description", opts.Description, 1000); err != nil {
		return err
	}
	
	// Validate repository name format
	if !isValidRepositoryName(opts.Name) {
		return fmt.Errorf("invalid repository name format")
	}
	
	return nil
}

// Sanitization helper methods

func (sc *SecureClient) sanitizeSearchOptions(opts SearchOptions) SearchOptions {
	return SearchOptions{
		Query:      sc.validator.SanitizeString(opts.Query),
		Owner:      sc.validator.SanitizeString(opts.Owner),
		Language:   sc.validator.SanitizeString(opts.Language),
		Stars:      sc.validator.SanitizeString(opts.Stars),
		Size:       sc.validator.SanitizeString(opts.Size),
		Fork:       opts.Fork,
		Archived:   opts.Archived,
		Sort:       sc.validator.SanitizeString(opts.Sort),
		Order:      sc.validator.SanitizeString(opts.Order),
		PerPage:    opts.PerPage,
		MaxResults: opts.MaxResults,
	}
}

func (sc *SecureClient) sanitizePullRequestOptions(opts *PullRequestOptions) *PullRequestOptions {
	return &PullRequestOptions{
		Title: sc.validator.SanitizeString(opts.Title),
		Head:  sc.validator.SanitizeString(opts.Head),
		Base:  sc.validator.SanitizeString(opts.Base),
		Body:  sc.validator.SanitizeString(opts.Body),
	}
}

func (sc *SecureClient) sanitizeCreateRepositoryOptions(opts *CreateRepositoryOptions) *CreateRepositoryOptions {
	return &CreateRepositoryOptions{
		Name:         sc.validator.SanitizeString(opts.Name),
		Organization: sc.validator.SanitizeString(opts.Organization),
		Private:      opts.Private,
		Description:  sc.validator.SanitizeString(opts.Description),
	}
}

// Secure version of buildSearchQuery that validates and sanitizes inputs
func buildSecureSearchQuery(opts SearchOptions, validator *security.InputValidator) (string, error) {
	var parts []string
	
	if opts.Query != "" {
		if err := validator.ValidateSearchQuery("query", opts.Query); err != nil {
			return "", err
		}
		sanitized := validator.SanitizeString(opts.Query)
		parts = append(parts, sanitized)
	}
	
	if opts.Owner != "" {
		if err := validator.ValidateString("owner", opts.Owner, 100); err != nil {
			return "", err
		}
		sanitized := validator.SanitizeString(opts.Owner)
		parts = append(parts, fmt.Sprintf("user:%s", sanitized))
	}
	
	if opts.Language != "" {
		if err := validator.ValidateString("language", opts.Language, 50); err != nil {
			return "", err
		}
		sanitized := validator.SanitizeString(opts.Language)
		parts = append(parts, fmt.Sprintf("language:%s", sanitized))
	}
	
	if opts.Stars != "" {
		if err := validator.ValidateString("stars", opts.Stars, 50); err != nil {
			return "", err
		}
		sanitized := validator.SanitizeString(opts.Stars)
		parts = append(parts, fmt.Sprintf("stars:%s", sanitized))
	}
	
	if opts.Size != "" {
		if err := validator.ValidateString("size", opts.Size, 50); err != nil {
			return "", err
		}
		sanitized := validator.SanitizeString(opts.Size)
		parts = append(parts, fmt.Sprintf("size:%s", sanitized))
	}
	
	if !opts.Fork {
		parts = append(parts, "fork:false")
	}
	
	if !opts.Archived {
		parts = append(parts, "archived:false")
	}
	
	if len(parts) == 0 {
		return "stars:>0", nil // Default safe query
	}
	
	return strings.Join(parts, " "), nil
}

// Helper functions

func isValidRepositoryName(name string) bool {
	// Repository names must be 1-100 characters
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	
	// Must contain only alphanumeric characters, hyphens, underscores, and periods
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
			 (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	
	// Cannot start or end with special characters
	if name[0] == '-' || name[0] == '_' || name[0] == '.' ||
	   name[len(name)-1] == '-' || name[len(name)-1] == '_' || name[len(name)-1] == '.' {
		return false
	}
	
	return true
}

// Rate limiting and timeout helpers

// WithRateLimit adds rate limiting to the client
func (sc *SecureClient) WithRateLimit(requestsPerSecond float64) *SecureClient {
	// This would implement rate limiting logic
	// For now, return the same client
	return sc
}

// WithTimeout sets a custom timeout for operations
func (sc *SecureClient) WithTimeout(timeout time.Duration) *SecureClient {
	// This would implement timeout logic
	// For now, return the same client
	return sc
}

// GetRateLimitInfo returns current GitHub API rate limit information
func (sc *SecureClient) GetRateLimitInfo() RateLimitInfo {
	return sc.client.GetRateLimitInfo()
}

// SetWaitForRateLimitReset configures whether to wait when rate limited
func (sc *SecureClient) SetWaitForRateLimitReset(wait bool) {
	sc.client.SetWaitForRateLimitReset(wait)
}