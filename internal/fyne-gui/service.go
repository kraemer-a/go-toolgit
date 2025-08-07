package fynegui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go-toolgit/internal/core/bitbucket"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/git"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/processor"
	"go-toolgit/internal/core/utils"
)

// Repository represents a Git repository with selection state
type Repository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	SSHURL   string `json:"ssh_url"`
	Private  bool   `json:"private"`
	Selected bool   `json:"selected"`
}

// ReplacementRule defines a string replacement rule
type ReplacementRule struct {
	Original      string `json:"original"`
	Replacement   string `json:"replacement"`
	Regex         bool   `json:"regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	WholeWord     bool   `json:"whole_word"`
}

// ProcessingOptions contains options for processing replacements
type ProcessingOptions struct {
	DryRun          bool     `json:"dry_run"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	PRTitle         string   `json:"pr_title"`
	PRBody          string   `json:"pr_body"`
	BranchPrefix    string   `json:"branch_prefix"`
}

// ProcessingResult contains the results of a processing operation
type ProcessingResult struct {
	Success           bool                         `json:"success"`
	Message           string                       `json:"message"`
	RepositoryResults []RepositoryResult           `json:"repository_results"`
	Diffs             map[string]map[string]string `json:"diffs,omitempty"`
}

// RepositoryResult contains the result for a single repository
type RepositoryResult struct {
	Repository   string   `json:"repository"`
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	PRUrl        string   `json:"pr_url,omitempty"`
	FilesChanged []string `json:"files_changed"`
	Replacements int      `json:"replacements"`
}

// ConfigData holds configuration data for the GUI
type ConfigData struct {
	Provider string `json:"provider"`

	// GitHub-specific fields
	GitHubURL    string `json:"github_url"`
	Token        string `json:"token"`
	Organization string `json:"organization"`
	Team         string `json:"team"`

	// Bitbucket-specific fields
	BitbucketURL string `json:"bitbucket_url"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Project      string `json:"project"`

	// Common fields
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	PRTitleTemplate string   `json:"pr_title_template"`
	PRBodyTemplate  string   `json:"pr_body_template"`
	BranchPrefix    string   `json:"branch_prefix"`

	// Migration settings
	MigrationSourceURL  string            `json:"migration_source_url,omitempty"`
	MigrationTargetOrg  string            `json:"migration_target_org,omitempty"`
	MigrationTargetRepo string            `json:"migration_target_repo,omitempty"`
	MigrationWebhookURL string            `json:"migration_webhook_url,omitempty"`
	MigrationTeams      map[string]string `json:"migration_teams,omitempty"`
}

// MigrationConfig holds configuration for repository migration
type MigrationConfig struct {
	SourceBitbucketURL   string            `json:"source_bitbucket_url"`
	TargetGitHubOrg      string            `json:"target_github_org"`
	TargetRepositoryName string            `json:"target_repository_name"`
	WebhookURL           string            `json:"webhook_url"`
	Teams                map[string]string `json:"teams"`
	DryRun               bool              `json:"dry_run"`
}

// MigrationStep represents a step in the migration process
type MigrationStep struct {
	Description string `json:"description"`
	Status      string `json:"status"` // "pending", "running", "completed", "failed"
	Progress    int    `json:"progress"`
	Message     string `json:"message"`
}

// MigrationResult contains the result of a migration operation
type MigrationResult struct {
	Success       bool            `json:"success"`
	Message       string          `json:"message"`
	GitHubRepoURL string          `json:"github_repo_url"`
	Steps         []MigrationStep `json:"steps"`
}

// RateLimitInfo contains GitHub API rate limit information
type RateLimitInfo = github.RateLimitInfo

// Service provides the business logic for the Fyne GUI
type Service struct {
	config       *config.Config
	logger       *utils.Logger
	githubClient *github.Client
	gitOps       *git.MemoryOperations
	engine       *processor.ReplacementEngine
}

// NewService creates a new Service instance
func NewService(cfg *config.Config, logger *utils.Logger) *Service {
	return &Service{
		config: cfg,
		logger: logger,
	}
}

// SaveConfig saves the current configuration to disk with automatic encryption
func (s *Service) SaveConfig(configData ConfigData) error {
	s.logger.Debug("SaveConfig called", "provider", configData.Provider)

	// Create secure config manager
	scm, err := config.NewSecureConfigManager()
	if err != nil {
		return fmt.Errorf("failed to create secure config manager: %w", err)
	}

	// Convert ConfigData to Config struct with provider-specific field mapping
	cfg := &config.Config{
		Provider: configData.Provider,
		Processing: config.ProcessingConfig{
			IncludePatterns: configData.IncludePatterns,
			ExcludePatterns: configData.ExcludePatterns,
			MaxWorkers:      s.config.Processing.MaxWorkers,
		},
		PullRequest: config.PullRequestConfig{
			TitleTemplate: configData.PRTitleTemplate,
			BodyTemplate:  configData.PRBodyTemplate,
			BranchPrefix:  configData.BranchPrefix,
			AutoMerge:     s.config.PullRequest.AutoMerge,
			DeleteBranch:  s.config.PullRequest.DeleteBranch,
		},
		Logging: s.config.Logging,
	}

	// Save BOTH provider configurations - preserve existing values if new ones are empty
	s.logger.Debug("Saving both provider configurations",
		"active_provider", configData.Provider,
		"github_url", configData.GitHubURL,
		"github_org", configData.Organization,
		"bitbucket_url", configData.BitbucketURL,
		"bitbucket_project", configData.Project)

	// Always set GitHub config - use new values if provided, otherwise keep existing
	githubBaseURL := configData.GitHubURL
	if githubBaseURL == "" {
		githubBaseURL = s.config.GitHub.BaseURL
	}
	githubToken := configData.Token
	if githubToken == "" {
		githubToken = s.config.GitHub.Token
	}
	githubOrg := configData.Organization
	if githubOrg == "" {
		githubOrg = s.config.GitHub.Org
	}
	githubTeam := configData.Team
	if githubTeam == "" {
		githubTeam = s.config.GitHub.Team
	}

	cfg.GitHub = config.GitHubConfig{
		BaseURL:          githubBaseURL,
		Token:            githubToken,
		Org:              githubOrg,
		Team:             githubTeam,
		Timeout:          s.config.GitHub.Timeout,
		MaxRetries:       s.config.GitHub.MaxRetries,
		WaitForRateLimit: s.config.GitHub.WaitForRateLimit,
	}

	// Always set Bitbucket config - use new values if provided, otherwise keep existing
	bitbucketBaseURL := configData.BitbucketURL
	if bitbucketBaseURL == "" {
		bitbucketBaseURL = s.config.Bitbucket.BaseURL
	}
	bitbucketUsername := configData.Username
	if bitbucketUsername == "" {
		bitbucketUsername = s.config.Bitbucket.Username
	}
	bitbucketPassword := configData.Password
	if bitbucketPassword == "" {
		bitbucketPassword = s.config.Bitbucket.Password
	}
	bitbucketProject := configData.Project
	if bitbucketProject == "" {
		bitbucketProject = s.config.Bitbucket.Project
	}

	cfg.Bitbucket = config.BitbucketConfig{
		BaseURL:    bitbucketBaseURL,
		Username:   bitbucketUsername,
		Password:   bitbucketPassword,
		Project:    bitbucketProject,
		Timeout:    s.config.Bitbucket.Timeout,
		MaxRetries: s.config.Bitbucket.MaxRetries,
	}

	// Save migration settings to viper (these are not encrypted)
	viper.Set("migration.source_url", configData.MigrationSourceURL)
	viper.Set("migration.target_org", configData.MigrationTargetOrg)
	viper.Set("migration.target_repo", configData.MigrationTargetRepo)
	viper.Set("migration.webhook_url", configData.MigrationWebhookURL)
	viper.Set("migration.teams", configData.MigrationTeams)

	// Always try current directory first
	currentDirConfig := "./config.yaml"
	s.logger.Debug("Attempting to save config to current directory", "path", currentDirConfig)
	if err := scm.SaveSecureConfigToFile(cfg, currentDirConfig); err == nil {
		s.logger.Info("Saved encrypted config to current directory", "path", currentDirConfig)
		return nil
	} else {
		s.logger.Debug("Failed to save to current directory", "error", err.Error())
	}

	// Current directory failed, try home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".go-toolgit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")
	if err := scm.SaveSecureConfigToFile(cfg, configPath); err != nil {
		return fmt.Errorf("failed to write encrypted config file: %w", err)
	}

	s.logger.Info("Saved encrypted config to home directory", "path", configPath)
	return nil
}

// InitializeServiceConfig updates the service's internal configuration and clients without saving to disk
func (s *Service) InitializeServiceConfig(configData ConfigData) error {
	// Update provider first
	s.config.Provider = configData.Provider

	// Update provider-specific fields (allow empty values to enable clearing)
	if configData.Provider == "github" {
		s.config.GitHub.BaseURL = configData.GitHubURL
		s.config.GitHub.Token = configData.Token
		s.config.GitHub.Org = configData.Organization
		s.config.GitHub.Team = configData.Team
		// Clear Bitbucket fields when using GitHub
		s.config.Bitbucket.BaseURL = ""
		s.config.Bitbucket.Username = ""
		s.config.Bitbucket.Password = ""
		s.config.Bitbucket.Project = ""
	} else if configData.Provider == "bitbucket" {
		s.config.Bitbucket.BaseURL = configData.BitbucketURL
		s.config.Bitbucket.Username = configData.Username
		s.config.Bitbucket.Password = configData.Password
		s.config.Bitbucket.Project = configData.Project
		// Preserve GitHub fields for migration support (don't clear them)
		s.config.GitHub.BaseURL = configData.GitHubURL
		s.config.GitHub.Token = configData.Token
		s.config.GitHub.Org = configData.Organization
		s.config.GitHub.Team = configData.Team
	}

	// Update processing patterns (allow empty to enable clearing)
	s.config.Processing.IncludePatterns = configData.IncludePatterns
	s.config.Processing.ExcludePatterns = configData.ExcludePatterns

	// Update pull request config (allow empty to enable clearing)
	s.config.PullRequest.TitleTemplate = configData.PRTitleTemplate
	s.config.PullRequest.BodyTemplate = configData.PRBodyTemplate
	s.config.PullRequest.BranchPrefix = configData.BranchPrefix

	// Initialize GitHub client for GitHub provider (required) or Bitbucket provider (optional, for migration)
	if configData.Provider == "github" {
		// GitHub provider requires GitHub client
		if s.config.GitHub.BaseURL == "" || s.config.GitHub.Token == "" {
			return fmt.Errorf("GitHub provider requires base URL and token")
		}

		// Create GitHub client config
		githubConfig := &github.Config{
			BaseURL:      s.config.GitHub.BaseURL,
			Token:        s.config.GitHub.Token,
			Timeout:      s.config.GitHub.Timeout,
			MaxRetries:   s.config.GitHub.MaxRetries,
			WaitForReset: s.config.GitHub.WaitForRateLimit,
		}

		// Initialize GitHub client
		var err error
		s.githubClient, err = github.NewClient(githubConfig)
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		// Initialize git memory operations with GitHub token
		s.gitOps = git.NewMemoryOperations(configData.Token)
	} else if configData.Provider == "bitbucket" {
		// Bitbucket provider: Create GitHub client for migration if credentials are available
		if configData.GitHubURL != "" && configData.Token != "" {
			// Create GitHub client for migration support using configData
			githubConfig := &github.Config{
				BaseURL:      configData.GitHubURL,
				Token:        configData.Token,
				Timeout:      s.config.GitHub.Timeout,
				MaxRetries:   s.config.GitHub.MaxRetries,
				WaitForReset: s.config.GitHub.WaitForRateLimit,
			}

			// Initialize GitHub client for migration
			var err error
			s.githubClient, err = github.NewClient(githubConfig)
			if err != nil {
				s.logger.Warn("Could not create GitHub client for migration support", "error", err)
				s.githubClient = nil
			} else {
				s.logger.Info("GitHub client created for migration support")
			}
		} else {
			// No GitHub credentials available - migration won't work but basic Bitbucket operations will
			s.logger.Info("No GitHub credentials provided - migration will not be available")
			s.githubClient = nil
		}

		// For Bitbucket, git operations will be initialized when needed for specific operations
		s.gitOps = nil
	} else {
		// Unknown provider
		s.githubClient = nil
		s.gitOps = nil
	}

	// Initialize replacement engine (will set rules later)
	var err2 error
	s.engine, err2 = processor.NewReplacementEngine([]processor.ReplacementRule{}, configData.IncludePatterns, configData.ExcludePatterns)
	if err2 != nil {
		return fmt.Errorf("failed to create replacement engine: %w", err2)
	}

	return nil
}

// UpdateConfig updates the service configuration and initializes clients (saves to disk)
func (s *Service) UpdateConfig(configData ConfigData) error {
	// Save configuration to disk first
	if err := s.SaveConfig(configData); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Initialize service configuration
	return s.InitializeServiceConfig(configData)
}

// ValidateAccess validates the access configuration for the selected provider
func (s *Service) ValidateAccess() error {
	ctx := context.Background()

	// Check which provider is configured and validate accordingly
	if s.config.Provider == "bitbucket" {
		return s.validateBitbucketAccess(ctx)
	}

	// Default to GitHub validation
	return s.validateGitHubAccess(ctx)
}

// validateGitHubAccess validates GitHub API access
func (s *Service) validateGitHubAccess(ctx context.Context) error {
	if s.githubClient == nil {
		return fmt.Errorf("GitHub client not initialized")
	}

	// Test GitHub API access - validate token access and org/team if specified
	err := s.githubClient.ValidateAccess(ctx, s.config.GitHub.Org, s.config.GitHub.Team)
	if err != nil {
		return fmt.Errorf("failed to validate GitHub access: %w", err)
	}

	return nil
}

// validateBitbucketAccess validates Bitbucket API access
func (s *Service) validateBitbucketAccess(ctx context.Context) error {
	// Create Bitbucket client for validation
	bitbucketClient, err := s.createBitbucketClient()
	if err != nil {
		return fmt.Errorf("failed to create Bitbucket client: %w", err)
	}

	// Test Bitbucket API access by fetching the project
	if s.config.Bitbucket.Project != "" {
		// Try to get project information
		project, err := bitbucketClient.GetProject(ctx, s.config.Bitbucket.Project)
		if err != nil {
			return fmt.Errorf("failed to validate Bitbucket access (project: %s): %w", s.config.Bitbucket.Project, err)
		}
		s.logger.Info("Bitbucket project validated successfully", "project", project.Key)
	} else {
		// If no project specified, just test basic API access by getting user info
		user, err := bitbucketClient.GetCurrentUser(ctx)
		if err != nil {
			return fmt.Errorf("failed to validate Bitbucket access (user authentication): %w", err)
		}
		s.logger.Info("Bitbucket access validated successfully", "user", user.DisplayName)
	}

	return nil
}

// ValidateConfiguration is an alias for ValidateAccess for compatibility
func (s *Service) ValidateConfiguration() error {
	return s.ValidateAccess()
}

// ListRepositories retrieves a list of repositories from GitHub
func (s *Service) ListRepositories() ([]Repository, error) {
	if s.githubClient == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}

	ctx := context.Background()
	var repos []*github.Repository
	var err error

	// If org and team are specified, get team repositories
	if s.config.GitHub.Org != "" && s.config.GitHub.Team != "" {
		// Get team info first
		team, err := s.githubClient.GetTeam(ctx, s.config.GitHub.Org, s.config.GitHub.Team)
		if err != nil {
			return nil, fmt.Errorf("failed to get team %s: %w", s.config.GitHub.Team, err)
		}

		// Get team repositories
		repos, err = s.githubClient.ListTeamRepositories(ctx, team)
		if err != nil {
			return nil, fmt.Errorf("failed to list team repositories: %w", err)
		}
	} else {
		// Get user repositories
		repos, err = s.githubClient.ListUserRepositories(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list user repositories: %w", err)
		}
	}

	var guiRepos []Repository
	for _, repo := range repos {
		guiRepos = append(guiRepos, Repository{
			Name:     repo.Name,
			FullName: repo.FullName,
			CloneURL: repo.CloneURL,
			SSHURL:   repo.SSHURL,
			Private:  repo.Private,
			Selected: false,
		})
	}

	return guiRepos, nil
}

// ProcessReplacements processes string replacements across repositories
func (s *Service) ProcessReplacements(rules []ReplacementRule, repos []Repository, options ProcessingOptions) (*ProcessingResult, error) {
	if s.engine == nil || s.gitOps == nil || s.githubClient == nil {
		return nil, fmt.Errorf("service components not initialized")
	}

	ctx := context.Background()

	// Convert GUI rules to processor rules
	var engineRules []processor.ReplacementRule
	for _, rule := range rules {
		engineRules = append(engineRules, processor.ReplacementRule{
			Original:      rule.Original,
			Replacement:   rule.Replacement,
			Regex:         rule.Regex,
			CaseSensitive: rule.CaseSensitive,
			WholeWord:     rule.WholeWord,
		})
	}

	// Create a new replacement engine with the rules
	engine, err := processor.NewReplacementEngine(engineRules, options.IncludePatterns, options.ExcludePatterns)
	if err != nil {
		return &ProcessingResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create replacement engine: %v", err),
		}, nil
	}

	// Create memory processor for efficient git operations
	memoryProcessor := processor.NewMemoryProcessor(engine, s.gitOps)

	// Process each repository
	var repoResults []RepositoryResult
	diffs := make(map[string]map[string]string)
	successCount := 0

	for _, repo := range repos {
		// Parse owner/repo from full name
		parts := strings.SplitN(repo.FullName, "/", 2)
		if len(parts) != 2 {
			repoResults = append(repoResults, RepositoryResult{
				Repository: repo.FullName,
				Success:    false,
				Message:    "Invalid repository name format",
			})
			continue
		}
		owner, repoName := parts[0], parts[1]

		// Process repository using memory-based git operations
		result, err := memoryProcessor.ProcessRepository(ctx, repo.CloneURL, repo.FullName, options.BranchPrefix, options.DryRun)
		if err != nil {
			repoResults = append(repoResults, RepositoryResult{
				Repository: repo.FullName,
				Success:    false,
				Message:    fmt.Sprintf("Processing failed: %v", err),
			})
			continue
		}

		// Create repository result
		repoResult := RepositoryResult{
			Repository:   result.Repository,
			Success:      result.Success,
			Message:      "Processing completed successfully",
			FilesChanged: result.FilesChanged,
			Replacements: result.Replacements,
		}

		if result.Error != nil {
			repoResult.Success = false
			repoResult.Message = result.Error.Error()
		}

		// If not dry run and changes were made, create PR
		if !options.DryRun && result.Success && len(result.FilesChanged) > 0 && result.Branch != "" {
			prOptions := &github.PullRequestOptions{
				Title: options.PRTitle,
				Head:  result.Branch,
				Base:  "main", // Default to main branch
				Body:  options.PRBody,
			}

			pr, err := s.githubClient.CreatePullRequest(ctx, owner, repoName, prOptions)
			if err != nil {
				repoResult.Message = fmt.Sprintf("Changes applied but failed to create PR: %v", err)
			} else {
				repoResult.PRUrl = pr.GetHTMLURL()
				repoResult.Message = "Changes applied and PR created"
			}
		}

		// For dry run, generate actual diffs from FileChanges
		if options.DryRun && len(result.FileChanges) > 0 {
			fileDiffs := make(map[string]string)
			for _, fileChange := range result.FileChanges {
				diff := s.generateDiffFromFileChange(fileChange)
				if diff != "" {
					fileDiffs[fileChange.FilePath] = diff
				}
			}
			if len(fileDiffs) > 0 {
				diffs[repo.FullName] = fileDiffs
			}
		}

		repoResults = append(repoResults, repoResult)
		if result.Success {
			successCount++
		}
	}

	// Create overall result
	overallSuccess := successCount == len(repos)
	message := fmt.Sprintf("Processed %d repositories, %d successful", len(repos), successCount)

	return &ProcessingResult{
		Success:           overallSuccess,
		Message:           message,
		RepositoryResults: repoResults,
		Diffs:             diffs,
	}, nil
}

// ValidateMigrationConfig validates migration configuration
func (s *Service) ValidateMigrationConfig(config MigrationConfig) error {
	if config.SourceBitbucketURL == "" {
		return fmt.Errorf("source Bitbucket URL is required")
	}
	if config.TargetGitHubOrg == "" {
		return fmt.Errorf("target GitHub organization is required")
	}
	if config.TargetRepositoryName == "" {
		return fmt.Errorf("target repository name is required")
	}

	// Validate GitHub access for migration target
	if s.githubClient == nil {
		return fmt.Errorf("GitHub client not initialized - migration requires GitHub credentials in the Configuration tab")
	}

	// Validate GitHub token has necessary permissions
	ctx := context.Background()
	err := s.githubClient.ValidateTokenAccess(ctx)
	if err != nil {
		return fmt.Errorf("cannot access GitHub with current token (migration target): %w", err)
	}

	s.logger.Info("Migration configuration validated successfully",
		"source", config.SourceBitbucketURL,
		"target_org", config.TargetGitHubOrg,
		"target_repo", config.TargetRepositoryName)

	return nil
}

// MigrateRepository performs repository migration from Bitbucket to GitHub
func (s *Service) MigrateRepository(config MigrationConfig) (*MigrationResult, error) {
	return s.MigrateRepositoryWithCallback(config, nil)
}

// MigrateRepositoryWithCallback performs repository migration with an optional live progress callback
func (s *Service) MigrateRepositoryWithCallback(config MigrationConfig, liveProgressCallback func([]MigrationStep)) (*MigrationResult, error) {
	ctx := context.Background()

	// Create Bitbucket client if Bitbucket configuration is available
	bitbucketClient, err := s.createBitbucketClient()
	if err != nil {
		// Log the error but don't fail immediately - we might still be able to use credentials directly
		s.logger.Warn("Could not create Bitbucket client, will try with credentials directly", "error", err)
		bitbucketClient = nil
	}

	// Create migration service with progress callback that updates both local steps and provides live updates
	var steps []MigrationStep
	progressCallback := func(step MigrationStep) {
		// Update the steps slice - find and update the matching step
		for i := range steps {
			if steps[i].Description == step.Description {
				steps[i] = step
				break
			}
		}
		s.logger.Info("Migration progress update", "step", step.Description, "status", step.Status, "progress", step.Progress)

		// Call live progress callback for GUI updates if provided
		if liveProgressCallback != nil {
			liveProgressCallback(steps)
		}
	}

	migrationService := NewMigrationService(s.githubClient, s.gitOps, &config, s.config.GitHub.Token, s.config.GitHub.BaseURL, progressCallback)

	// Always set Bitbucket credentials if available, even if client creation failed
	// This allows the migration to work with credentials directly for git operations
	if s.config.Bitbucket.Username != "" && s.config.Bitbucket.Password != "" {
		migrationService.SetBitbucketCredentials(s.config.Bitbucket.Username, s.config.Bitbucket.Password)
	}

	// Set the Bitbucket client if it was successfully created
	if bitbucketClient != nil {
		migrationService.SetBitbucketClient(bitbucketClient)
	}

	// Initialize steps for tracking
	steps = []MigrationStep{
		{Description: "Validating source repository", Status: "pending", Progress: 0},
		{Description: "Creating target repository", Status: "pending", Progress: 0},
		{Description: "Cloning source repository", Status: "pending", Progress: 0},
		{Description: "Pushing to target repository", Status: "pending", Progress: 0},
		{Description: "Configuring teams", Status: "pending", Progress: 0},
		{Description: "Setting up webhooks", Status: "pending", Progress: 0},
		{Description: "Triggering pipeline", Status: "pending", Progress: 0},
	}

	// Perform the actual migration
	result, err := migrationService.MigrateRepositoryImpl(ctx)
	if err != nil {
		s.logger.Error("Migration failed", "error", err)
		return result, err
	}

	s.logger.Info("Migration completed", "success", result.Success, "message", result.Message)
	return result, nil
}

// createBitbucketClient creates a Bitbucket client from service configuration
func (s *Service) createBitbucketClient() (*bitbucket.Client, error) {
	// Use the service's current configuration (which includes GUI values)
	// instead of loading from file to ensure consistency
	if s.config.Bitbucket.BaseURL == "" || s.config.Bitbucket.Username == "" || s.config.Bitbucket.Password == "" {
		// Return nil client if Bitbucket is not configured - migration service will handle this
		return nil, fmt.Errorf("Bitbucket configuration incomplete - base_url, username, and password/app_password required")
	}

	timeout := s.config.Bitbucket.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	maxRetries := s.config.Bitbucket.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	bitbucketConfig := &bitbucket.Config{
		BaseURL:    s.config.Bitbucket.BaseURL,
		Username:   s.config.Bitbucket.Username,
		Password:   s.config.Bitbucket.Password, // Already decrypted if loaded from file
		Timeout:    timeout,
		MaxRetries: maxRetries,
	}

	return bitbucket.NewClient(bitbucketConfig)
}

// ReadConfigFromFile reads configuration from a file with automatic decryption
func (s *Service) ReadConfigFromFile() (*ConfigData, error) {
	// Initialize viper if not already configured
	initializeViper()

	// Use secure config loading to automatically decrypt sensitive fields
	cfg, err := config.LoadSecure()
	if err != nil {
		return nil, fmt.Errorf("failed to load secure configuration: %w", err)
	}

	// Use provider from config, default to github if not set
	provider := cfg.Provider
	if provider == "" {
		provider = "github"
	}

	configData := &ConfigData{
		Provider:        provider,
		IncludePatterns: cfg.Processing.IncludePatterns,
		ExcludePatterns: cfg.Processing.ExcludePatterns,
		PRTitleTemplate: cfg.PullRequest.TitleTemplate,
		PRBodyTemplate:  cfg.PullRequest.BodyTemplate,
		BranchPrefix:    cfg.PullRequest.BranchPrefix,

		// Load migration settings from viper if they exist
		MigrationSourceURL:  viper.GetString("migration.source_url"),
		MigrationTargetOrg:  viper.GetString("migration.target_org"),
		MigrationTargetRepo: viper.GetString("migration.target_repo"),
		MigrationWebhookURL: viper.GetString("migration.webhook_url"),
		MigrationTeams:      viper.GetStringMapString("migration.teams"),

		// Always load BOTH provider configurations regardless of active provider
		// GitHub configuration
		GitHubURL:    cfg.GitHub.BaseURL,
		Token:        cfg.GitHub.Token, // This is now automatically decrypted
		Organization: cfg.GitHub.Org,
		Team:         cfg.GitHub.Team,

		// Bitbucket configuration
		BitbucketURL: cfg.Bitbucket.BaseURL,
		Username:     cfg.Bitbucket.Username,
		Password:     cfg.Bitbucket.Password, // This is now automatically decrypted
		Project:      cfg.Bitbucket.Project,
	}

	return configData, nil
}

// GetRateLimitInfo retrieves GitHub API rate limit information
func (s *Service) GetRateLimitInfo() (*RateLimitInfo, error) {
	// Rate limiting is only applicable for GitHub provider
	if s.config.Provider != "github" {
		return nil, fmt.Errorf("rate limit information is only available for GitHub provider (current: %s)", s.config.Provider)
	}

	if s.githubClient == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}

	ctx := context.Background()
	rateLimitInfo, err := s.githubClient.GetLiveRateLimitInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limits: %w", err)
	}

	return &rateLimitInfo, nil
}

// initializeViper sets up viper configuration paths if not already configured
func initializeViper() {
	// Check if viper is already configured
	if viper.ConfigFileUsed() != "" {
		return
	}

	// Set defaults
	setDefaults()

	// Configure search paths (current directory first to match save priority)
	viper.AddConfigPath(".")
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(home + "/.go-toolgit")
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Set environment variable prefix
	viper.SetEnvPrefix("GITHUB_REPLACE")
	viper.AutomaticEnv()

	// Try to read the config file
	viper.ReadInConfig()
}

// generateDiffFromFileChange generates a unified diff from FileChange data
func (s *Service) generateDiffFromFileChange(fileChange *processor.FileChange) string {
	if fileChange == nil || len(fileChange.StringChanges) == 0 {
		return ""
	}

	var diff strings.Builder

	// Write file header
	diff.WriteString(fmt.Sprintf("--- %s (original)\n", fileChange.FilePath))
	diff.WriteString(fmt.Sprintf("+++ %s (modified)\n", fileChange.FilePath))

	// Generate diff for each change
	for i, change := range fileChange.StringChanges {
		// Add hunk header for each change
		diff.WriteString(fmt.Sprintf("@@ -%d,1 +%d,1 @@ Change %d of %d\n",
			change.LineNumber, change.LineNumber, i+1, len(fileChange.StringChanges)))

		// If we have context, show it first
		if change.Context != "" {
			// Show context line with the original string highlighted
			contextWithOriginal := change.Context
			diff.WriteString(fmt.Sprintf("- %s\n", contextWithOriginal))

			// Show context line with the replacement string
			contextWithReplacement := strings.Replace(change.Context, change.Original, change.Replacement, -1)
			diff.WriteString(fmt.Sprintf("+ %s\n", contextWithReplacement))
		} else {
			// No context, just show the raw change
			diff.WriteString(fmt.Sprintf("- %s\n", change.Original))
			diff.WriteString(fmt.Sprintf("+ %s\n", change.Replacement))
		}

		diff.WriteString("\n")
	}

	return diff.String()
}

// setDefaults sets default configuration values
func setDefaults() {
	viper.SetDefault("github.base_url", "https://api.github.com")
	viper.SetDefault("github.timeout", "30s")
	viper.SetDefault("github.max_retries", 3)
	viper.SetDefault("github.wait_for_rate_limit", true)
	viper.SetDefault("processing.include_patterns", []string{"*.go", "*.java", "*.js", "*.py", "*.ts", "*.jsx", "*.tsx"})
	viper.SetDefault("processing.exclude_patterns", []string{"vendor/*", "node_modules/*", "*.min.js", ".git/*"})
	viper.SetDefault("processing.max_workers", 4)
	viper.SetDefault("pull_request.title_template", "chore: automated string replacement")
	viper.SetDefault("pull_request.body_template", "Automated string replacement performed by go-toolgit tool.")
	viper.SetDefault("pull_request.branch_prefix", "auto-replace")
	viper.SetDefault("pull_request.auto_merge", false)
	viper.SetDefault("pull_request.delete_branch", true)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
}
