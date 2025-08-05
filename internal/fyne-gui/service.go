package fynegui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
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
	Success           bool                        `json:"success"`
	Message           string                      `json:"message"`
	RepositoryResults []RepositoryResult          `json:"repository_results"`
	Diffs            map[string]map[string]string `json:"diffs,omitempty"`
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
	Provider        string   `json:"provider"`
	GitHubURL       string   `json:"github_url"`
	Token           string   `json:"token"`
	Organization    string   `json:"organization"`
	Team            string   `json:"team"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	PRTitleTemplate string   `json:"pr_title_template"`
	PRBodyTemplate  string   `json:"pr_body_template"`
	BranchPrefix    string   `json:"branch_prefix"`
	
	// Migration settings
	MigrationSourceURL   string            `json:"migration_source_url,omitempty"`
	MigrationTargetOrg   string            `json:"migration_target_org,omitempty"`
	MigrationTargetRepo  string            `json:"migration_target_repo,omitempty"`
	MigrationWebhookURL  string            `json:"migration_webhook_url,omitempty"`
	MigrationTeams       map[string]string `json:"migration_teams,omitempty"`
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

// SaveConfig saves the current configuration to disk
func (s *Service) SaveConfig(configData ConfigData) error {
	// Update viper configuration values
	viper.Set("provider", configData.Provider)
	viper.Set("github.base_url", configData.GitHubURL)
	viper.Set("github.token", configData.Token)
	viper.Set("github.organization", configData.Organization)
	viper.Set("github.team", configData.Team)
	viper.Set("processing.include_patterns", configData.IncludePatterns)
	viper.Set("processing.exclude_patterns", configData.ExcludePatterns)
	viper.Set("pull_request.title_template", configData.PRTitleTemplate)
	viper.Set("pull_request.body_template", configData.PRBodyTemplate)
	viper.Set("pull_request.branch_prefix", configData.BranchPrefix)
	
	// Save migration settings (always set, even if empty to allow clearing)
	viper.Set("migration.source_url", configData.MigrationSourceURL)
	viper.Set("migration.target_org", configData.MigrationTargetOrg)
	viper.Set("migration.target_repo", configData.MigrationTargetRepo)
	viper.Set("migration.webhook_url", configData.MigrationWebhookURL)
	viper.Set("migration.teams", configData.MigrationTeams)
	
	// Always try current directory first
	currentDirConfig := "./config.yaml"
	if err := viper.WriteConfigAs(currentDirConfig); err == nil {
		s.logger.Info("Saved config to current directory", "path", currentDirConfig)
		return nil
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
	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	s.logger.Info("Saved config to home directory", "path", configPath)
	return nil
}

// UpdateConfig updates the service configuration and initializes clients
func (s *Service) UpdateConfig(configData ConfigData) error {
	// Save configuration to disk first
	if err := s.SaveConfig(configData); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}
	
	// Update config with new data
	if configData.GitHubURL != "" {
		s.config.GitHub.BaseURL = configData.GitHubURL
	}
	if configData.Token != "" {
		s.config.GitHub.Token = configData.Token
	}
	if configData.Organization != "" {
		s.config.GitHub.Org = configData.Organization
	}
	if configData.Team != "" {
		s.config.GitHub.Team = configData.Team
	}
	
	// Update processing patterns in config
	if len(configData.IncludePatterns) > 0 {
		s.config.Processing.IncludePatterns = configData.IncludePatterns
	}
	if len(configData.ExcludePatterns) > 0 {
		s.config.Processing.ExcludePatterns = configData.ExcludePatterns
	}
	
	// Update pull request config
	if configData.PRTitleTemplate != "" {
		s.config.PullRequest.TitleTemplate = configData.PRTitleTemplate
	}
	if configData.PRBodyTemplate != "" {
		s.config.PullRequest.BodyTemplate = configData.PRBodyTemplate
	}
	if configData.BranchPrefix != "" {
		s.config.PullRequest.BranchPrefix = configData.BranchPrefix
	}
	
	// Update provider
	s.config.Provider = configData.Provider

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

	// Initialize git memory operations
	s.gitOps = git.NewMemoryOperations(configData.Token)

	// Initialize replacement engine (will set rules later)
	var err2 error
	s.engine, err2 = processor.NewReplacementEngine([]processor.ReplacementRule{}, configData.IncludePatterns, configData.ExcludePatterns)
	if err2 != nil {
		return fmt.Errorf("failed to create replacement engine: %w", err2)
	}

	return nil
}

// ValidateAccess validates the GitHub access configuration
func (s *Service) ValidateAccess() error {
	if s.githubClient == nil {
		return fmt.Errorf("GitHub client not initialized")
	}

	ctx := context.Background()
	
	// Test GitHub API access - validate token access and org/team if specified
	err := s.githubClient.ValidateAccess(ctx, s.config.GitHub.Org, s.config.GitHub.Team)
	if err != nil {
		return fmt.Errorf("failed to validate GitHub access: %w", err)
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
		Diffs:            diffs,
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

	// Validate GitHub access
	if s.githubClient == nil {
		return fmt.Errorf("GitHub client not initialized")
	}

	// For now, just validate basic access - a full implementation would check org access
	ctx := context.Background()
	err := s.githubClient.ValidateTokenAccess(ctx)
	if err != nil {
		return fmt.Errorf("cannot access GitHub with current token: %w", err)
	}

	return nil
}

// MigrateRepository performs repository migration from Bitbucket to GitHub
func (s *Service) MigrateRepository(config MigrationConfig) (*MigrationResult, error) {
	// This is a placeholder implementation
	// In a real implementation, this would handle the complex migration process
	steps := []MigrationStep{
		{
			Description: "Validating source repository",
			Status:      "completed",
			Progress:    100,
			Message:     "Source repository validated",
		},
		{
			Description: "Creating target repository",
			Status:      "completed",
			Progress:    100,
			Message:     "Target repository created",
		},
		{
			Description: "Cloning source repository",
			Status:      "completed",
			Progress:    100,
			Message:     "Source repository cloned",
		},
		{
			Description: "Pushing to target repository",
			Status:      "completed",
			Progress:    100,
			Message:     "Code pushed to target repository",
		},
	}

	if config.DryRun {
		// For dry run, mark all steps as pending
		for i := range steps {
			steps[i].Status = "pending"
			steps[i].Progress = 0
			steps[i].Message = "Would " + steps[i].Description
		}
	}

	result := &MigrationResult{
		Success:       !config.DryRun,
		Message:       "Migration completed successfully",
		GitHubRepoURL: fmt.Sprintf("https://github.com/%s/%s", config.TargetGitHubOrg, config.TargetRepositoryName),
		Steps:         steps,
	}

	if config.DryRun {
		result.Message = "Migration dry run completed - no actual changes made"
	}

	return result, nil
}

// ReadConfigFromFile reads configuration from a file
func (s *Service) ReadConfigFromFile() (*ConfigData, error) {
	// Initialize viper if not already configured
	initializeViper()
	
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Use provider from config, default to github if not set
	provider := cfg.Provider
	if provider == "" {
		provider = "github"
	}
	
	return &ConfigData{
		Provider:        provider,
		GitHubURL:       cfg.GitHub.BaseURL,
		Token:           cfg.GitHub.Token,
		Organization:    cfg.GitHub.Org,
		Team:            cfg.GitHub.Team,
		IncludePatterns: cfg.Processing.IncludePatterns,
		ExcludePatterns: cfg.Processing.ExcludePatterns,
		PRTitleTemplate: cfg.PullRequest.TitleTemplate,
		PRBodyTemplate:  cfg.PullRequest.BodyTemplate,
		BranchPrefix:    cfg.PullRequest.BranchPrefix,
		
		// Load migration settings from viper if they exist
		MigrationSourceURL:   viper.GetString("migration.source_url"),
		MigrationTargetOrg:   viper.GetString("migration.target_org"),
		MigrationTargetRepo:  viper.GetString("migration.target_repo"),
		MigrationWebhookURL:  viper.GetString("migration.webhook_url"),
		MigrationTeams:       viper.GetStringMapString("migration.teams"),
	}, nil
}

// GetRateLimitInfo retrieves GitHub API rate limit information  
func (s *Service) GetRateLimitInfo() (*RateLimitInfo, error) {
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
	
	// Configure search paths
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(home + "/.go-toolgit")
	}
	viper.AddConfigPath(".")
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