package fynegui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go-toolgit/internal/core/bitbucket"
	"go-toolgit/internal/core/git"
	"go-toolgit/internal/core/github"
)

// MigrationService handles repository migration from Bitbucket to GitHub
type MigrationService struct {
	githubClient      *github.Client
	bitbucketClient   *bitbucket.Client
	gitOps            *git.MemoryOperations
	config            *MigrationConfig
	githubToken       string
	bitbucketUsername string
	bitbucketPassword string
	progressCallback  func(step MigrationStep)
}

// NewMigrationService creates a new migration service
func NewMigrationService(githubClient *github.Client, gitOps *git.MemoryOperations, config *MigrationConfig, githubToken string, progressCallback func(step MigrationStep)) *MigrationService {
	return &MigrationService{
		githubClient:     githubClient,
		gitOps:           gitOps,
		config:           config,
		githubToken:      githubToken,
		progressCallback: progressCallback,
	}
}

// MigrateRepositoryImpl performs the actual repository migration
func (ms *MigrationService) MigrateRepositoryImpl(ctx context.Context) (*MigrationResult, error) {
	steps := []MigrationStep{
		{Description: "Validating source repository", Status: "pending", Progress: 0},
		{Description: "Creating target repository", Status: "pending", Progress: 0},
		{Description: "Cloning source repository", Status: "pending", Progress: 0},
		{Description: "Pushing to target repository", Status: "pending", Progress: 0},
		{Description: "Configuring teams", Status: "pending", Progress: 0},
		{Description: "Setting up webhooks", Status: "pending", Progress: 0},
	}

	result := &MigrationResult{
		Success: false,
		Steps:   steps,
	}

	// Step 1: Validate source repository
	ms.updateStep(&steps[0], "running", 0, "Connecting to Bitbucket...")
	bitbucketRepo, err := ms.validateBitbucketSource(ctx)
	if err != nil {
		ms.updateStep(&steps[0], "failed", 0, fmt.Sprintf("Failed: %v", err))
		result.Message = fmt.Sprintf("Source validation failed: %v", err)
		return result, err
	}
	ms.updateStep(&steps[0], "completed", 100, "Source repository validated")

	// Step 2: Create target repository (skip in dry run)
	ms.updateStep(&steps[1], "running", 0, "Creating GitHub repository...")
	var githubRepo *github.Repository
	if !ms.config.DryRun {
		githubRepo, err = ms.createGitHubTarget(ctx)
		if err != nil {
			ms.updateStep(&steps[1], "failed", 0, fmt.Sprintf("Failed: %v", err))
			result.Message = fmt.Sprintf("GitHub repository creation failed: %v", err)
			return result, err
		}
		ms.updateStep(&steps[1], "completed", 100, "Target repository created")
	} else {
		ms.updateStep(&steps[1], "completed", 100, "Would create target repository")
	}

	// Step 3: Clone source repository
	ms.updateStep(&steps[2], "running", 0, "Cloning from Bitbucket...")
	var memoryRepo *git.MemoryRepository
	if !ms.config.DryRun {
		memoryRepo, err = ms.cloneFromBitbucket(ctx, bitbucketRepo)
		if err != nil {
			ms.updateStep(&steps[2], "failed", 0, fmt.Sprintf("Failed: %v", err))
			result.Message = fmt.Sprintf("Repository cloning failed: %v", err)
			return result, err
		}
		ms.updateStep(&steps[2], "completed", 100, "Repository cloned successfully")
	} else {
		ms.updateStep(&steps[2], "completed", 100, "Would clone source repository")
	}

	// Step 4: Push to target repository (skip in dry run)
	ms.updateStep(&steps[3], "running", 0, "Pushing to GitHub...")
	if !ms.config.DryRun && githubRepo != nil && memoryRepo != nil {
		err = ms.pushToGitHub(ctx, memoryRepo, githubRepo)
		if err != nil {
			ms.updateStep(&steps[3], "failed", 0, fmt.Sprintf("Failed: %v", err))
			result.Message = fmt.Sprintf("Push to GitHub failed: %v", err)
			return result, err
		}
		ms.updateStep(&steps[3], "completed", 100, "Code pushed to GitHub")
	} else {
		ms.updateStep(&steps[3], "completed", 100, "Would push to target repository")
	}

	// Step 5: Configure teams (skip in dry run)
	ms.updateStep(&steps[4], "running", 0, "Configuring team access...")
	if !ms.config.DryRun && len(ms.config.Teams) > 0 {
		err = ms.configureTeams(ctx)
		if err != nil {
			ms.updateStep(&steps[4], "failed", 0, fmt.Sprintf("Failed: %v", err))
			result.Message = fmt.Sprintf("Team configuration failed: %v", err)
			return result, err
		}
		ms.updateStep(&steps[4], "completed", 100, fmt.Sprintf("Configured %d team(s)", len(ms.config.Teams)))
	} else if len(ms.config.Teams) > 0 {
		ms.updateStep(&steps[4], "completed", 100, fmt.Sprintf("Would configure %d team(s)", len(ms.config.Teams)))
	} else {
		ms.updateStep(&steps[4], "completed", 100, "No teams to configure")
	}

	// Step 6: Set up webhooks (skip in dry run)
	ms.updateStep(&steps[5], "running", 0, "Setting up webhooks...")
	if !ms.config.DryRun && ms.config.WebhookURL != "" {
		err = ms.setupWebhooks(ctx)
		if err != nil {
			ms.updateStep(&steps[5], "failed", 0, fmt.Sprintf("Failed: %v", err))
			result.Message = fmt.Sprintf("Webhook setup failed: %v", err)
			return result, err
		}
		ms.updateStep(&steps[5], "completed", 100, "Webhook configured")
	} else if ms.config.WebhookURL != "" {
		ms.updateStep(&steps[5], "completed", 100, "Would configure webhook")
	} else {
		ms.updateStep(&steps[5], "completed", 100, "No webhook to configure")
	}

	// Success!
	result.Success = true
	if ms.config.DryRun {
		result.Message = "Migration dry run completed - no actual changes made"
	} else {
		result.Message = "Migration completed successfully"
		if githubRepo != nil {
			result.GitHubRepoURL = fmt.Sprintf("https://github.com/%s", githubRepo.FullName)
		}
	}

	return result, nil
}

// validateBitbucketSource validates access to the source Bitbucket repository
func (ms *MigrationService) validateBitbucketSource(ctx context.Context) (*bitbucket.Repository, error) {
	// Parse the Bitbucket URL to extract project and repo info
	sourceURL, err := url.Parse(ms.config.SourceBitbucketURL)
	if err != nil {
		return nil, fmt.Errorf("invalid source URL format: %w", err)
	}

	// Extract project key and repo slug from URL path
	// Expected formats:
	// 1. /projects/PROJECT/repos/REPO (Bitbucket Server REST API)
	// 2. /scm/PROJECT/REPO.git (Bitbucket Server SCM)
	// 3. /PROJECT/REPO.git (Direct SSH format)
	path := strings.TrimPrefix(sourceURL.Path, "/")
	parts := strings.Split(path, "/")

	var projectKey, repoSlug string
	if len(parts) >= 4 && parts[0] == "projects" && parts[2] == "repos" {
		// Format: /projects/PROJECT/repos/REPO
		projectKey = parts[1]
		repoSlug = parts[3]
	} else if len(parts) >= 3 && parts[0] == "scm" {
		// Format: /scm/PROJECT/REPO.git
		projectKey = parts[1]
		repoSlug = strings.TrimSuffix(parts[2], ".git")
	} else if len(parts) >= 2 {
		// Format: /PROJECT/REPO.git (direct SSH format)
		projectKey = parts[0]
		repoSlug = strings.TrimSuffix(parts[1], ".git")
		// If repoSlug is still empty after trimming, use the full part
		if repoSlug == "" {
			repoSlug = parts[1]
		}
	} else {
		return nil, fmt.Errorf("unable to parse project and repository from URL: %s (supported formats: /projects/PROJECT/repos/REPO, /scm/PROJECT/REPO.git, /PROJECT/REPO.git)", ms.config.SourceBitbucketURL)
	}

	// Initialize Bitbucket client if not already done
	if ms.bitbucketClient == nil {
		// We need to create a Bitbucket client from the config
		// This would require Bitbucket credentials from the main config
		return nil, fmt.Errorf("Bitbucket client not configured - Bitbucket authentication required")
	}

	// Validate access by getting repository info
	repo, err := ms.bitbucketClient.GetRepository(ctx, projectKey, repoSlug)
	if err != nil {
		return nil, fmt.Errorf("cannot access source repository: %w", err)
	}

	return repo, nil
}

// createGitHubTarget creates the target repository on GitHub
func (ms *MigrationService) createGitHubTarget(ctx context.Context) (*github.Repository, error) {
	opts := &github.CreateRepositoryOptions{
		Name:         ms.config.TargetRepositoryName,
		Organization: ms.config.TargetGitHubOrg,
		Private:      true, // Default to private, can be made configurable
		Description:  fmt.Sprintf("Migrated from Bitbucket: %s", ms.config.SourceBitbucketURL),
	}

	repo, err := ms.githubClient.CreateRepository(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub repository: %w", err)
	}

	return repo, nil
}

// cloneFromBitbucket clones the source repository using go-git
func (ms *MigrationService) cloneFromBitbucket(ctx context.Context, bitbucketRepo *bitbucket.Repository) (*git.MemoryRepository, error) {
	// Use the clone URL from the Bitbucket repository
	cloneURL := bitbucketRepo.CloneURL
	if cloneURL == "" {
		return nil, fmt.Errorf("no clone URL available for repository")
	}

	// Convert SSH URL to HTTPS if needed
	httpsURL, err := ms.convertSSHToHTTPS(cloneURL)
	if err != nil {
		return nil, fmt.Errorf("failed to convert SSH URL to HTTPS: %w", err)
	}

	// Validate Bitbucket credentials
	if ms.bitbucketUsername == "" || ms.bitbucketPassword == "" {
		return nil, fmt.Errorf("Bitbucket credentials not configured - username and password required for authentication")
	}

	// Create git operations instance (token not used for basic auth)
	bitbucketGitOps := git.NewMemoryOperations("")

	// Clone using Bitbucket basic authentication
	memoryRepo, err := bitbucketGitOps.CloneRepositoryWithBasicAuth(ctx, httpsURL, bitbucketRepo.FullName, ms.bitbucketUsername, ms.bitbucketPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s with basic auth: %w", bitbucketRepo.FullName, err)
	}

	return memoryRepo, nil
}

// pushToGitHub pushes the cloned repository to GitHub
func (ms *MigrationService) pushToGitHub(ctx context.Context, memoryRepo *git.MemoryRepository, githubRepo *github.Repository) error {
	// Get GitHub token from git operations (we need access to this)
	// For now, we'll assume the token is available through the service configuration

	// Push all branches to the GitHub repository
	err := memoryRepo.PushAllBranchesToRemote(ctx, githubRepo.CloneURL, ms.getGitHubToken())
	if err != nil {
		return fmt.Errorf("failed to push all branches to GitHub: %w", err)
	}

	return nil
}

// getGitHubToken retrieves the GitHub token
func (ms *MigrationService) getGitHubToken() string {
	return ms.githubToken
}

// configureTeams adds teams to the GitHub repository
func (ms *MigrationService) configureTeams(ctx context.Context) error {
	for teamName, permission := range ms.config.Teams {
		err := ms.githubClient.AddTeamToRepository(ctx, ms.config.TargetGitHubOrg, teamName, ms.config.TargetRepositoryName, permission)
		if err != nil {
			return fmt.Errorf("failed to add team %s with permission %s: %w", teamName, permission, err)
		}
	}
	return nil
}

// setupWebhooks configures webhooks for the GitHub repository
func (ms *MigrationService) setupWebhooks(ctx context.Context) error {
	if ms.config.WebhookURL == "" {
		return nil // No webhook to configure
	}

	opts := &github.CreateWebhookOptions{
		URL:         ms.config.WebhookURL,
		ContentType: "json",
		Events:      []string{"push", "pull_request", "issues"},
		Active:      true,
	}

	_, err := ms.githubClient.CreateWebhook(ctx, ms.config.TargetGitHubOrg, ms.config.TargetRepositoryName, opts)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	return nil
}

// updateStep updates a migration step and calls the progress callback
func (ms *MigrationService) updateStep(step *MigrationStep, status string, progress int, message string) {
	step.Status = status
	step.Progress = progress
	step.Message = message

	if ms.progressCallback != nil {
		ms.progressCallback(*step)
	}
}

// SetBitbucketClient sets the Bitbucket client for the migration service
func (ms *MigrationService) SetBitbucketClient(client *bitbucket.Client) {
	ms.bitbucketClient = client
}

// SetBitbucketCredentials sets the Bitbucket authentication credentials
func (ms *MigrationService) SetBitbucketCredentials(username, password string) {
	ms.bitbucketUsername = username
	ms.bitbucketPassword = password
}

// convertSSHToHTTPS converts SSH URL to HTTPS format for authentication
func (ms *MigrationService) convertSSHToHTTPS(sshURL string) (string, error) {
	// Parse the SSH URL
	parsedURL, err := url.Parse(sshURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse SSH URL: %w", err)
	}

	// Handle SSH URL format: ssh://git@host:port/path/repo.git
	if parsedURL.Scheme == "ssh" {
		// Extract hostname and port
		host := parsedURL.Hostname()
		port := parsedURL.Port()

		// Convert to HTTPS format
		httpsURL := fmt.Sprintf("https://%s", host)
		if port != "" && port != "22" && port != "443" {
			// Only add port if it's not the default ports
			httpsURL = fmt.Sprintf("https://%s:%s", host, port)
		}

		// Add the path (remove leading slash and git@ user)
		path := strings.TrimPrefix(parsedURL.Path, "/")
		httpsURL = fmt.Sprintf("%s/%s", httpsURL, path)

		return httpsURL, nil
	}

	// If it's already HTTPS or another format, return as-is
	return sshURL, nil
}
