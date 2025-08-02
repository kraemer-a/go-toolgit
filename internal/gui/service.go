package gui

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-toolgit/internal/core/bitbucket"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/git"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/processor"
	"go-toolgit/internal/core/utils"
	
	gogithub "github.com/google/go-github/v66/github"
	"gopkg.in/yaml.v3"
)

type Service struct {
	config    *config.Config
	logger    *utils.Logger
	github    *github.Client
	bitbucket *bitbucket.Client
}

type ConfigData struct {
	Provider         string   `json:"provider"` // "github" or "bitbucket"
	GitHubURL        string   `json:"github_url"`
	Token            string   `json:"token"`
	Organization     string   `json:"organization"`
	Team             string   `json:"team"`
	BitbucketURL     string   `json:"bitbucket_url"`
	BitbucketUser    string   `json:"bitbucket_username"`
	BitbucketPass    string   `json:"bitbucket_password"`
	BitbucketProject string   `json:"bitbucket_project"`
	IncludePatterns  []string `json:"include_patterns"`
	ExcludePatterns  []string `json:"exclude_patterns"`
	PRTitleTemplate  string   `json:"pr_title_template"`
	PRBodyTemplate   string   `json:"pr_body_template"`
	BranchPrefix     string   `json:"branch_prefix"`
}

type ReplacementRule struct {
	Original      string `json:"original"`
	Replacement   string `json:"replacement"`
	Regex         bool   `json:"regex"`
	CaseSensitive bool   `json:"case_sensitive"`
	WholeWord     bool   `json:"whole_word"`
}

type Repository struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
	Private  bool   `json:"private"`
	Selected bool   `json:"selected"`
}

type ProcessingOptions struct {
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	DryRun          bool     `json:"dry_run"`
	PRTitle         string   `json:"pr_title"`
	PRBody          string   `json:"pr_body"`
	BranchPrefix    string   `json:"branch_prefix"`
}

type ProcessingResult struct {
	Success           bool                         `json:"success"`
	Message           string                       `json:"message"`
	RepositoryResults []RepositoryResult           `json:"repository_results"`
	Stats             *processor.ReplacementStats  `json:"stats"`
	Diffs             map[string]map[string]string `json:"diffs,omitempty"` // repo -> file -> diff content
}

type RepositoryResult struct {
	Repository   string            `json:"repository"`
	Success      bool              `json:"success"`
	Message      string            `json:"message"`
	PRUrl        string            `json:"pr_url,omitempty"`
	FilesChanged []string          `json:"files_changed"`
	Replacements int               `json:"replacements"`
	Diffs        map[string]string `json:"diffs,omitempty"` // file -> diff content
}

func NewService(cfg *config.Config, logger *utils.Logger) *Service {
	return &Service{
		config: cfg,
		logger: logger,
	}
}

func (s *Service) GetConfig() ConfigData {
	return ConfigData{
		Provider:         s.config.Provider,
		GitHubURL:        s.config.GitHub.BaseURL,
		Token:            s.config.GitHub.Token,
		Organization:     s.config.GitHub.Org,
		Team:             s.config.GitHub.Team,
		BitbucketURL:     s.config.Bitbucket.BaseURL,
		BitbucketUser:    s.config.Bitbucket.Username,
		BitbucketPass:    s.config.Bitbucket.Password,
		BitbucketProject: s.config.Bitbucket.Project,
	}
}

func (s *Service) UpdateConfig(cfg ConfigData) error {
	// Set provider
	if cfg.Provider == "" {
		cfg.Provider = "github" // default
	}
	s.config.Provider = cfg.Provider

	// Update GitHub config
	s.config.GitHub.BaseURL = cfg.GitHubURL
	s.config.GitHub.Token = cfg.Token
	s.config.GitHub.Org = cfg.Organization
	s.config.GitHub.Team = cfg.Team

	// Update Bitbucket config
	s.config.Bitbucket.BaseURL = cfg.BitbucketURL
	s.config.Bitbucket.Username = cfg.BitbucketUser
	s.config.Bitbucket.Password = cfg.BitbucketPass
	s.config.Bitbucket.Project = cfg.BitbucketProject

	// Update processing patterns
	s.config.Processing.IncludePatterns = cfg.IncludePatterns
	s.config.Processing.ExcludePatterns = cfg.ExcludePatterns

	// Update pull request templates
	s.config.PullRequest.TitleTemplate = cfg.PRTitleTemplate
	s.config.PullRequest.BodyTemplate = cfg.PRBodyTemplate
	s.config.PullRequest.BranchPrefix = cfg.BranchPrefix

	// Use search validation if org/team are not provided for GitHub
	var err error
	if s.config.Provider == "github" && (s.config.GitHub.Org == "" || s.config.GitHub.Team == "") {
		err = s.config.ValidateForSearch()
	} else {
		err = s.config.Validate()
	}
	if err != nil {
		return utils.NewValidationError("configuration validation failed", err)
	}

	// Initialize the appropriate client based on provider
	switch s.config.Provider {
	case "github":
		s.github, err = github.NewClient(&github.Config{
			BaseURL:    s.config.GitHub.BaseURL,
			Token:      s.config.GitHub.Token,
			Timeout:    s.config.GitHub.Timeout,
			MaxRetries: s.config.GitHub.MaxRetries,
		})
		if err != nil {
			return utils.NewAuthError("failed to create GitHub client", err)
		}
	case "bitbucket":
		s.bitbucket, err = bitbucket.NewClient(&bitbucket.Config{
			BaseURL:    s.config.Bitbucket.BaseURL,
			Username:   s.config.Bitbucket.Username,
			Password:   s.config.Bitbucket.Password,
			Timeout:    s.config.Bitbucket.Timeout,
			MaxRetries: s.config.Bitbucket.MaxRetries,
		})
		if err != nil {
			return utils.NewAuthError("failed to create Bitbucket client", err)
		}
	default:
		return fmt.Errorf("unsupported provider: %s", s.config.Provider)
	}

	// Save configuration to ./config.yaml file
	err = s.saveConfigToFile()
	if err != nil {
		return fmt.Errorf("failed to save configuration to file: %w", err)
	}

	s.logger.Info("Configuration updated and saved successfully", "provider", s.config.Provider)
	return nil
}

// saveConfigToFile saves the current configuration to ./config.yaml
func (s *Service) saveConfigToFile() error {
	configFile := "./config.yaml"
	
	// Convert config to YAML
	yamlData, err := yaml.Marshal(s.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}
	
	// Write to file
	err = os.WriteFile(configFile, yamlData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	s.logger.Info("Configuration saved to file", "file", configFile)
	return nil
}

// ReadConfigFromFile reads configuration from ./config.yaml and returns ConfigData for GUI
func (s *Service) ReadConfigFromFile() (*ConfigData, error) {
	configFile := "./config.yaml"
	
	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		s.logger.Info("Config file does not exist, returning empty config", "file", configFile)
		return &ConfigData{
			Provider:  "github", // default
			GitHubURL: "https://api.github.com", // default
		}, nil
	}
	
	// Read file
	yamlData, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse YAML into config struct
	var cfg config.Config
	err = yaml.Unmarshal(yamlData, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config YAML: %w", err)
	}
	
	// Convert to ConfigData for GUI
	configData := &ConfigData{
		Provider:         cfg.Provider,
		GitHubURL:        cfg.GitHub.BaseURL,
		Token:            cfg.GitHub.Token,
		Organization:     cfg.GitHub.Org,
		Team:             cfg.GitHub.Team,
		BitbucketURL:     cfg.Bitbucket.BaseURL,
		BitbucketUser:    cfg.Bitbucket.Username,
		BitbucketPass:    cfg.Bitbucket.Password,
		BitbucketProject: cfg.Bitbucket.Project,
		IncludePatterns:  cfg.Processing.IncludePatterns,
		ExcludePatterns:  cfg.Processing.ExcludePatterns,
		PRTitleTemplate:  cfg.PullRequest.TitleTemplate,
		PRBodyTemplate:   cfg.PullRequest.BodyTemplate,
		BranchPrefix:     cfg.PullRequest.BranchPrefix,
	}
	
	s.logger.Info("Configuration loaded from file", "file", configFile, "provider", cfg.Provider)
	return configData, nil
}

func (s *Service) ValidateAccess() error {
	ctx, timeout := context.WithTimeout(context.Background(), 30*time.Second)
	defer timeout()

	switch s.config.Provider {
	case "github":
		if s.github == nil {
			return fmt.Errorf("GitHub client not initialized - please update configuration first")
		}
		// If org/team are provided, use team-based validation
		if s.config.GitHub.Org != "" && s.config.GitHub.Team != "" {
			return s.github.ValidateAccess(ctx, s.config.GitHub.Org, s.config.GitHub.Team)
		} else {
			// For owner-only access, just validate basic API access
			return s.github.ValidateTokenAccess(ctx)
		}
	case "bitbucket":
		if s.bitbucket == nil {
			return fmt.Errorf("Bitbucket client not initialized - please update configuration first")
		}
		return s.bitbucket.ValidateAccess(ctx)
	default:
		return fmt.Errorf("unsupported provider: %s", s.config.Provider)
	}
}

func (s *Service) ListRepositories() ([]Repository, error) {
	ctx, timeout := context.WithTimeout(context.Background(), 60*time.Second)
	defer timeout()

	switch s.config.Provider {
	case "github":
		if s.github == nil {
			return nil, fmt.Errorf("GitHub client not initialized - please update configuration first")
		}

		var repositories []*github.Repository
		var err error

		// If org/team are provided, use team-based listing
		if s.config.GitHub.Org != "" && s.config.GitHub.Team != "" {
			team, err := s.github.GetTeam(ctx, s.config.GitHub.Org, s.config.GitHub.Team)
			if err != nil {
				return nil, utils.NewNetworkError("failed to get team information", err)
			}

			repositories, err = s.github.ListTeamRepositories(ctx, team.ID)
			if err != nil {
				return nil, utils.NewNetworkError("failed to list team repositories", err)
			}
		} else {
			// For owner-only access, list user's repositories
			repositories, err = s.github.ListUserRepositories(ctx)
			if err != nil {
				return nil, utils.NewNetworkError("failed to list user repositories", err)
			}
		}

		result := make([]Repository, len(repositories))
		for i, repo := range repositories {
			result[i] = Repository{
				ID:       repo.ID,
				Name:     repo.Name,
				FullName: repo.FullName,
				CloneURL: repo.CloneURL,
				Private:  repo.Private,
				Selected: false,
			}
		}

		s.logger.Info("Retrieved GitHub repositories", "count", len(result))
		return result, nil

	case "bitbucket":
		if s.bitbucket == nil {
			return nil, fmt.Errorf("Bitbucket client not initialized - please update configuration first")
		}

		repositories, err := s.bitbucket.ListProjectRepositories(ctx, s.config.Bitbucket.Project)
		if err != nil {
			return nil, utils.NewNetworkError("failed to list project repositories", err)
		}

		result := make([]Repository, len(repositories))
		for i, repo := range repositories {
			result[i] = Repository{
				ID:       repo.ID,
				Name:     repo.Name,
				FullName: repo.FullName,
				CloneURL: repo.CloneURL,
				Private:  repo.Private,
				Selected: false,
			}
		}

		s.logger.Info("Retrieved Bitbucket repositories", "count", len(result))
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", s.config.Provider)
	}
}

func (s *Service) ProcessReplacements(
	rules []ReplacementRule,
	selectedRepos []Repository,
	options ProcessingOptions,
) (*ProcessingResult, error) {
	// Validate client initialization based on provider
	switch s.config.Provider {
	case "github":
		if s.github == nil {
			return nil, fmt.Errorf("GitHub client not initialized - please update configuration first")
		}
	case "bitbucket":
		if s.bitbucket == nil {
			return nil, fmt.Errorf("Bitbucket client not initialized - please update configuration first")
		}
	default:
		return nil, fmt.Errorf("unsupported provider: %s", s.config.Provider)
	}

	s.logger.Info("Starting replacement processing",
		"rules_count", len(rules),
		"repos_count", len(selectedRepos),
		"dry_run", options.DryRun)

	processorRules := make([]processor.ReplacementRule, len(rules))
	for i, rule := range rules {
		
		processorRules[i] = processor.ReplacementRule{
			Original:      rule.Original,
			Replacement:   rule.Replacement,
			Regex:         rule.Regex,
			CaseSensitive: rule.CaseSensitive,
			WholeWord:     rule.WholeWord,
		}
	}

	includePatterns := options.IncludePatterns
	if len(includePatterns) == 0 {
		includePatterns = s.config.Processing.IncludePatterns
	}

	excludePatterns := options.ExcludePatterns
	if len(excludePatterns) == 0 {
		excludePatterns = s.config.Processing.ExcludePatterns
	}
	
	s.logger.Info("Using file patterns", "includePatterns", includePatterns, "excludePatterns", excludePatterns)

	engine, err := processor.NewReplacementEngine(processorRules, includePatterns, excludePatterns)
	if err != nil {
		return nil, utils.NewProcessingError("failed to create replacement engine", err)
	}

	result := &ProcessingResult{
		Success:           true,
		Message:           "Processing completed successfully",
		RepositoryResults: make([]RepositoryResult, 0),
	}

	// Initialize diffs map for dry run
	if options.DryRun {
		result.Diffs = make(map[string]map[string]string)
	}

	for _, repo := range selectedRepos {
		s.logger.Info("Checking repository", "repo", repo.FullName, "selected", repo.Selected)
		if !repo.Selected {
			s.logger.Info("Skipping unselected repository", "repo", repo.FullName)
			continue
		}

		s.logger.Info("Processing repository", "repo", repo.FullName)
		repoResult := s.processRepository(repo, engine, options)
		result.RepositoryResults = append(result.RepositoryResults, repoResult)

		// Collect diffs from repository results for dry run
		if options.DryRun && repoResult.Diffs != nil {
			result.Diffs[repo.FullName] = repoResult.Diffs
		}

		if !repoResult.Success {
			result.Success = false
		}
	}

	stats := engine.GetStats()
	result.Stats = stats

	if result.Success {
		result.Message = fmt.Sprintf("Successfully processed %d repositories", len(result.RepositoryResults))
	} else {
		successCount := 0
		for _, r := range result.RepositoryResults {
			if r.Success {
				successCount++
			}
		}
		result.Message = fmt.Sprintf("Processed %d/%d repositories successfully", successCount, len(result.RepositoryResults))
	}

	s.logger.Info("Replacement processing completed",
		"success", result.Success,
		"files_processed", stats.FilesProcessed,
		"files_modified", stats.FilesModified,
		"total_replacements", stats.Replacements)

	return result, nil
}

func (s *Service) processRepository(
	repo Repository,
	engine *processor.ReplacementEngine,
	options ProcessingOptions,
) RepositoryResult {
	s.logger.Info("Processing repository", "repo", repo.FullName)

	result := RepositoryResult{
		Repository:   repo.FullName,
		Success:      true,
		Message:      "Repository processed successfully",
		FilesChanged: []string{},
		Replacements: 0,
	}

	if options.DryRun {
		// Dry run - just simulate the replacements
		result.Diffs = s.simulateReplacements(repo, engine, options)

		// Count files and replacements from diffs
		for fileName := range result.Diffs {
			result.FilesChanged = append(result.FilesChanged, fileName)
			// Count lines that start with + or - (excluding headers)
			lines := strings.Split(result.Diffs[fileName], "\n")
			for _, line := range lines {
				if (strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++")) ||
					(strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---")) {
					result.Replacements++
				}
			}
		}

		if len(result.FilesChanged) == 0 {
			result.Message = "No files matched the include patterns or contained the search terms"
		} else {
			result.Message = fmt.Sprintf("Would modify %d files with %d changes",
				len(result.FilesChanged), result.Replacements/2) // Divide by 2 since each replacement has + and -
		}
	} else {
		// Actual processing - apply changes via git operations
		err := s.applyRepositoryChanges(repo, engine, options, &result)
		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("Failed to apply changes: %v", err)
			s.logger.Error("Failed to apply repository changes", "repo", repo.FullName, "error", err)
		}
	}

	return result
}

func (s *Service) simulateReplacements(repo Repository, engine *processor.ReplacementEngine, options ProcessingOptions) map[string]string {
	diffs := make(map[string]string)

	// Get the actual replacement rules from the engine
	rules := engine.GetRules()
	s.logger.Debug("Processing with rules", "rules_count", len(rules), "repo", repo.FullName)
	for i, rule := range rules {
		s.logger.Debug("Rule", "index", i, "original", rule.Original, "replacement", rule.Replacement, "regex", rule.Regex, "case_sensitive", rule.CaseSensitive)
	}

	// Clone repository and get files using git operations (same as apply changes)
	actualFiles, err := s.getRepositoryFilesViaGit(repo, options.IncludePatterns, options.ExcludePatterns)
	if err != nil {
		s.logger.Error("Failed to get repository files via git", "repo", repo.FullName, "error", err)
		return diffs
	}

	// Process all actual files (already filtered by readActualFiles)
	for fileName, content := range actualFiles {
		s.logger.Debug("Processing file", "fileName", fileName, "contentSize", len(content))

		// Apply replacement rules
		modifiedContent := content
		hasChanges := false

		for _, rule := range rules {
			s.logger.Debug("Applying rule", "fileName", fileName, "original", rule.Original, "replacement", rule.Replacement, "regex", rule.Regex, "caseSensitive", rule.CaseSensitive)

			if rule.Regex {
				// Handle regex replacements
				re, err := regexp.Compile(rule.Original)
				if err != nil {
					s.logger.Debug("Failed to compile regex", "pattern", rule.Original, "error", err)
					continue
				}
				if re.MatchString(modifiedContent) {
					s.logger.Debug("Regex match found", "fileName", fileName, "pattern", rule.Original)
					modifiedContent = re.ReplaceAllString(modifiedContent, rule.Replacement)
					hasChanges = true
				} else {
					s.logger.Debug("No regex match", "fileName", fileName, "pattern", rule.Original)
				}
			} else {
				// Handle literal string replacements
				searchStr := rule.Original
				
				// Build regex pattern for whole word support
				pattern := regexp.QuoteMeta(searchStr)
				if rule.WholeWord {
					pattern = `\b` + pattern + `\b`
				}
				
				var re *regexp.Regexp
				if !rule.CaseSensitive {
					re = regexp.MustCompile("(?i)" + pattern)
				} else {
					re = regexp.MustCompile(pattern)
				}
				
				if re.MatchString(modifiedContent) {
					s.logger.Debug("Literal match found", "fileName", fileName, "searchStr", searchStr, "wholeWord", rule.WholeWord)
					modifiedContent = re.ReplaceAllString(modifiedContent, rule.Replacement)
					hasChanges = true
				} else {
					s.logger.Debug("No literal match", "fileName", fileName, "searchStr", searchStr, "wholeWord", rule.WholeWord)
				}
			}
		}

		// Generate diff if there are changes
		if hasChanges {
			s.logger.Debug("Generating diff for file", "fileName", fileName)
			diffs[fileName] = s.generateDiff(fileName, content, modifiedContent)
		} else {
			s.logger.Debug("No changes found for file", "fileName", fileName)
		}
	}

	s.logger.Debug("Simulation complete", "diffsGenerated", len(diffs))
	return diffs
}

func (s *Service) generateDiff(fileName, original, modified string) string {
	originalLines := strings.Split(original, "\n")
	modifiedLines := strings.Split(modified, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- a/%s\n", fileName))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", fileName))

	// Simple diff algorithm - find changed lines
	maxLines := len(originalLines)
	if len(modifiedLines) > maxLines {
		maxLines = len(modifiedLines)
	}

	changes := []string{}

	for i := 0; i < maxLines; i++ {
		origLine := ""
		modLine := ""

		if i < len(originalLines) {
			origLine = originalLines[i]
		}
		if i < len(modifiedLines) {
			modLine = modifiedLines[i]
		}

		if origLine != modLine {
			if origLine != "" {
				changes = append(changes, fmt.Sprintf("-%s", origLine))
			}
			if modLine != "" {
				changes = append(changes, fmt.Sprintf("+%s", modLine))
			}
		} else if origLine != "" {
			changes = append(changes, fmt.Sprintf(" %s", origLine))
		}
	}

	// Add hunk header
	if len(changes) > 0 {
		diff.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(originalLines), len(modifiedLines)))
		for _, change := range changes {
			diff.WriteString(change + "\n")
		}
	}

	return diff.String()
}

func (s *Service) GetDefaultIncludePatterns() []string {
	return s.config.Processing.IncludePatterns
}

func (s *Service) GetDefaultExcludePatterns() []string {
	return s.config.Processing.ExcludePatterns
}

func (s *Service) GetCurrentProvider() string {
	if s.config.Provider == "" {
		return "github" // default
	}
	return s.config.Provider
}

func (s *Service) GetSupportedProviders() []string {
	return []string{"github", "bitbucket"}
}

type SearchCriteria struct {
	Query           string `json:"query"`
	Owner           string `json:"owner"`
	Language        string `json:"language"`
	Stars           string `json:"stars"`
	Size            string `json:"size"`
	IncludeForks    bool   `json:"include_forks"`
	IncludeArchived bool   `json:"include_archived"`
	Sort            string `json:"sort"`
	Order           string `json:"order"`
	MaxResults      int    `json:"max_results"`
}

// Migration types
type MigrationConfig struct {
	SourceBitbucketURL   string            `json:"source_bitbucket_url"`
	TargetGitHubOrg      string            `json:"target_github_org"`
	TargetRepositoryName string            `json:"target_repository_name"`
	WebhookURL           string            `json:"webhook_url"`
	Teams                map[string]string `json:"teams"` // team_name -> permission
	DryRun               bool              `json:"dry_run"`
}

type MigrationStep struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"` // pending, running, completed, failed
	Message     string `json:"message"`
	Progress    int    `json:"progress"` // 0-100
}

type MigrationResult struct {
	Success         bool             `json:"success"`
	Message         string           `json:"message"`
	Steps           []MigrationStep  `json:"steps"`
	GitHubRepoURL   string           `json:"github_repo_url,omitempty"`
	CreatedTeams    []string         `json:"created_teams,omitempty"`
	CreatedWebhooks []string         `json:"created_webhooks,omitempty"`
}

func (s *Service) SearchRepositories(criteria SearchCriteria) ([]Repository, error) {
	// Only GitHub supports search currently
	if s.config.Provider != "github" {
		return nil, fmt.Errorf("repository search is only supported for GitHub provider")
	}
	if s.github == nil {
		return nil, fmt.Errorf("GitHub client not initialized - please update configuration first")
	}

	ctx, timeout := context.WithTimeout(context.Background(), 60*time.Second)
	defer timeout()

	searchOpts := github.SearchOptions{
		Query:      criteria.Query,
		Owner:      criteria.Owner,
		Language:   criteria.Language,
		Stars:      criteria.Stars,
		Size:       criteria.Size,
		Fork:       criteria.IncludeForks,
		Archived:   criteria.IncludeArchived,
		Sort:       criteria.Sort,
		Order:      criteria.Order,
		MaxResults: criteria.MaxResults,
	}

	if searchOpts.MaxResults <= 0 {
		searchOpts.MaxResults = 100 // Default limit
	}

	repositories, err := s.github.SearchRepositories(ctx, searchOpts)
	if err != nil {
		return nil, utils.NewNetworkError("failed to search repositories", err)
	}

	result := make([]Repository, len(repositories))
	for i, repo := range repositories {
		result[i] = Repository{
			ID:       repo.ID,
			Name:     repo.Name,
			FullName: repo.FullName,
			CloneURL: repo.CloneURL,
			Private:  repo.Private,
			Selected: false,
		}
	}

	s.logger.Info("Retrieved repositories via search", "count", len(result))
	return result, nil
}

func (s *Service) ProcessSearchReplacements(
	criteria SearchCriteria,
	rules []ReplacementRule,
	options ProcessingOptions,
) (*ProcessingResult, error) {
	// Only GitHub supports search currently
	if s.config.Provider != "github" {
		return nil, fmt.Errorf("repository search is only supported for GitHub provider")
	}
	if s.github == nil {
		return nil, fmt.Errorf("GitHub client not initialized - please update configuration first")
	}

	s.logger.Info("Starting search-based replacement processing",
		"search_query", criteria.Query,
		"search_owner", criteria.Owner,
		"rules_count", len(rules),
		"dry_run", options.DryRun)

	// First, search for repositories
	repositories, err := s.SearchRepositories(criteria)
	if err != nil {
		return nil, err
	}

	if len(repositories) == 0 {
		return &ProcessingResult{
			Success:           false,
			Message:           "No repositories found matching search criteria",
			RepositoryResults: []RepositoryResult{},
		}, nil
	}

	// Mark all repositories as selected for processing
	for i := range repositories {
		repositories[i].Selected = true
	}

	// Process the found repositories
	return s.ProcessReplacements(rules, repositories, options)
}

func (s *Service) fetchRepositoryFiles(repo Repository, includePatterns, excludePatterns []string) (map[string]string, error) {
	files := make(map[string]string)

	s.logger.Debug("Fetching repository files", "repo", repo.FullName, "includePatterns", includePatterns, "excludePatterns", excludePatterns)

	// Only GitHub is supported for now
	if s.config.Provider != "github" {
		return nil, fmt.Errorf("repository file fetching is only supported for GitHub provider")
	}
	if s.github == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}

	ctx, timeout := context.WithTimeout(context.Background(), 60*time.Second)
	defer timeout()

	// Get repository tree to find all files
	tree, err := s.github.GetRepositoryTree(ctx, repo.FullName, "HEAD", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository tree: %w", err)
	}

	s.logger.Debug("Retrieved repository tree", "repo", repo.FullName, "files_count", len(tree.Entries))

	for _, entry := range tree.Entries {
		// Skip directories and non-blob entries
		if entry.Type != "blob" {
			continue
		}

		fileName := entry.Path

		// Check if file should be excluded
		excluded := false
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(fileName)); matched {
				excluded = true
				break
			}
			if matched, _ := filepath.Match(pattern, fileName); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check if file matches include patterns
		matched := len(includePatterns) == 0 // If no include patterns, include all
		for _, pattern := range includePatterns {
			if m, _ := filepath.Match(pattern, filepath.Base(fileName)); m {
				matched = true
				break
			}
			if m, _ := filepath.Match(pattern, fileName); m {
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		// Skip very large files (> 1MB)
		if entry.Size > 1024*1024 {
			s.logger.Debug("Skipping large file", "fileName", fileName, "size", entry.Size)
			continue
		}

		// Fetch file content
		content, err := s.github.GetFileContent(ctx, repo.FullName, fileName, "HEAD")
		if err != nil {
			s.logger.Debug("Failed to fetch file content", "fileName", fileName, "error", err)
			continue
		}

		// Skip binary files (simple check for null bytes)
		if strings.Contains(content, "\x00") {
			s.logger.Debug("Skipping binary file", "fileName", fileName)
			continue
		}

		files[fileName] = content
		s.logger.Debug("Added repository file to processing", "fileName", fileName, "size", len(content))
	}

	s.logger.Debug("Found repository files for processing", "count", len(files), "repo", repo.FullName)
	return files, nil
}

func (s *Service) ReadActualFiles(includePatterns, excludePatterns []string) map[string]string {
	files := make(map[string]string)
	currentDir := "."

	s.logger.Debug("Reading actual files", "includePatterns", includePatterns, "excludePatterns", excludePatterns, "currentDir", currentDir)

	err := filepath.WalkDir(currentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		if d.IsDir() {
			// Skip certain directories
			dirName := d.Name()
			if dirName == ".git" || dirName == "node_modules" || dirName == "vendor" ||
				dirName == "build" || dirName == ".idea" || dirName == "frontend" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be excluded
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, d.Name()); matched {
				return nil
			}
			if matched, _ := filepath.Match(pattern, path); matched {
				return nil
			}
		}

		// Check if file matches include patterns
		matched := len(includePatterns) == 0 // If no include patterns, include all
		for _, pattern := range includePatterns {
			if m, _ := filepath.Match(pattern, d.Name()); m {
				matched = true
				break
			}
			if m, _ := filepath.Match(pattern, path); m {
				matched = true
				break
			}
		}

		if !matched {
			return nil
		}

		// Skip binary files and very large files
		if s.isBinaryFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			s.logger.Debug("Failed to read file", "path", path, "error", err)
			return nil
		}

		// Skip very large files (> 1MB)
		if len(content) > 1024*1024 {
			return nil
		}

		files[path] = string(content)
		s.logger.Debug("Added file to processing", "path", path, "size", len(content))
		return nil
	})

	if err != nil {
		s.logger.Debug("Error walking directory", "error", err)
	}

	s.logger.Debug("Found files for processing", "count", len(files), "files", func() []string {
		var fileList []string
		for path := range files {
			fileList = append(fileList, path)
		}
		return fileList
	}())

	return files
}

func (s *Service) isBinaryFile(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return true
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return true
	}

	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}

	return false
}

func (s *Service) applyRepositoryChanges(repo Repository, engine *processor.ReplacementEngine, options ProcessingOptions, result *RepositoryResult) error {
	s.logger.Info("Applying changes to repository using git operations", "repo", repo.FullName)

	// Only GitHub is supported for now
	if s.config.Provider != "github" {
		return fmt.Errorf("repository changes are only supported for GitHub provider")
	}
	if s.github == nil {
		return fmt.Errorf("GitHub client not initialized")
	}

	ctx, timeout := context.WithTimeout(context.Background(), 300*time.Second) // 5 minutes
	defer timeout()

	// Initialize git operations
	gitOps := git.NewMemoryOperations(s.config.GitHub.Token)

	// Clone repository into memory
	memRepo, err := gitOps.CloneRepository(ctx, repo.CloneURL, repo.FullName)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	s.logger.Debug("Repository cloned successfully", "repo", repo.FullName)

	// Get all files from the repository
	files, err := memRepo.ListFiles()
	if err != nil {
		return fmt.Errorf("failed to list repository files: %w", err)
	}

	s.logger.Debug("Found files in repository", "count", len(files))

	// Filter files based on include/exclude patterns
	var filteredFiles []git.FileInfo
	for _, file := range files {
		// Check if file should be excluded
		excluded := false
		for _, pattern := range options.ExcludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(file.Path)); matched {
				excluded = true
				break
			}
			if matched, _ := filepath.Match(pattern, file.Path); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check if file matches include patterns
		matched := len(options.IncludePatterns) == 0 // If no include patterns, include all
		for _, pattern := range options.IncludePatterns {
			if m, _ := filepath.Match(pattern, filepath.Base(file.Path)); m {
				matched = true
				break
			}
			if m, _ := filepath.Match(pattern, file.Path); m {
				matched = true
				break
			}
		}

		if matched {
			filteredFiles = append(filteredFiles, file)
		}
	}

	if len(filteredFiles) == 0 {
		result.Message = "No files matched the include patterns"
		return nil
	}

	s.logger.Debug("Filtered files for processing", "count", len(filteredFiles))

	// Apply replacements to files
	rules := engine.GetRules()
	var modifiedFiles []git.FileInfo
	totalReplacements := 0

	for _, file := range filteredFiles {
		content := string(file.Content)
		modifiedContent := content
		hasChanges := false
		fileReplacements := 0

		for _, rule := range rules {
			s.logger.Debug("Applying rule to file", "file", file.Path, "original", rule.Original, "replacement", rule.Replacement)

			if rule.Regex {
				// Handle regex replacements
				re, err := regexp.Compile(rule.Original)
				if err != nil {
					s.logger.Debug("Failed to compile regex", "pattern", rule.Original, "error", err)
					continue
				}
				matches := re.FindAllString(modifiedContent, -1)
				if len(matches) > 0 {
					modifiedContent = re.ReplaceAllString(modifiedContent, rule.Replacement)
					hasChanges = true
					fileReplacements += len(matches)
				}
			} else {
				// Handle literal string replacements
				searchStr := rule.Original
				
				// Build regex pattern for whole word support
				pattern := regexp.QuoteMeta(searchStr)
				if rule.WholeWord {
					pattern = `\b` + pattern + `\b`
				}
				
				var re *regexp.Regexp
				if !rule.CaseSensitive {
					re = regexp.MustCompile("(?i)" + pattern)
				} else {
					re = regexp.MustCompile(pattern)
				}
				
				matches := re.FindAllString(modifiedContent, -1)
				if len(matches) > 0 {
					modifiedContent = re.ReplaceAllString(modifiedContent, rule.Replacement)
					hasChanges = true
					fileReplacements += len(matches)
				}
			}
		}

		if hasChanges {
			s.logger.Debug("File modified", "file", file.Path, "replacements", fileReplacements)
			modifiedFile := git.FileInfo{
				Path:    file.Path,
				Content: []byte(modifiedContent),
				Mode:    file.Mode,
			}
			modifiedFiles = append(modifiedFiles, modifiedFile)
			result.FilesChanged = append(result.FilesChanged, file.Path)
			totalReplacements += fileReplacements
		}
	}

	if len(modifiedFiles) == 0 {
		result.Message = "No changes were needed"
		return nil
	}

	result.Replacements = totalReplacements
	s.logger.Info("Applied replacements", "files_modified", len(modifiedFiles), "total_replacements", totalReplacements)

	// Update files in the repository
	err = memRepo.UpdateFiles(modifiedFiles)
	if err != nil {
		return fmt.Errorf("failed to update files: %w", err)
	}

	// Check if there are actually changes
	hasChanges, err := memRepo.HasChanges()
	if err != nil {
		return fmt.Errorf("failed to check for changes: %w", err)
	}

	if !hasChanges {
		result.Message = "No changes were detected after applying replacements"
		return nil
	}

	// Create a new branch name
	branchName := gitOps.GenerateBranchName(options.BranchPrefix)
	commitMessage := fmt.Sprintf("%s\n\nApplied %d replacements across %d files", options.PRTitle, totalReplacements, len(modifiedFiles))

	// Create branch and commit changes
	err = memRepo.CreateBranchAndCommit(branchName, commitMessage)
	if err != nil {
		return fmt.Errorf("failed to create branch and commit: %w", err)
	}

	s.logger.Info("Created branch and committed changes", "branch", branchName)

	// Push the branch to remote
	err = memRepo.Push(ctx, branchName)
	if err != nil {
		return fmt.Errorf("failed to push branch: %w", err)
	}

	s.logger.Info("Pushed branch to remote", "branch", branchName)

	// Get repository info to find the default branch
	repoInfo, err := s.github.GetRepository(ctx, repo.FullName)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	defaultBranch := repoInfo.GetDefaultBranch()
	if defaultBranch == "" {
		defaultBranch = "main" // fallback
	}

	s.logger.Info("Using default branch for PR", "branch", defaultBranch, "repo", repo.FullName)

	// Create pull request
	parts := strings.SplitN(repo.FullName, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository full name: %s", repo.FullName)
	}
	owner, repoName := parts[0], parts[1]

	prOptions := &github.PullRequestOptions{
		Title: options.PRTitle,
		Head:  branchName,
		Base:  defaultBranch,
		Body:  options.PRBody,
	}

	pr, err := s.github.CreatePullRequest(ctx, owner, repoName, prOptions)
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}

	prURL := pr.GetHTMLURL()
	s.logger.Info("Created pull request", "pr_url", prURL, "pr_number", pr.GetNumber())

	result.PRUrl = prURL
	result.Message = fmt.Sprintf("Successfully processed %d files with %d replacements and created PR #%d",
		len(modifiedFiles), totalReplacements, pr.GetNumber())

	s.logger.Debug("Final result", "pr_url", result.PRUrl, "success", result.Success)

	return nil
}

func (s *Service) getRepositoryFilesViaGit(repo Repository, includePatterns, excludePatterns []string) (map[string]string, error) {
	s.logger.Info("Getting repository files via git", "repo", repo.FullName, "includePatterns", includePatterns, "excludePatterns", excludePatterns)

	// Only GitHub is supported for now
	if s.config.Provider != "github" {
		return nil, fmt.Errorf("repository file fetching is only supported for GitHub provider")
	}
	if s.github == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}

	ctx, timeout := context.WithTimeout(context.Background(), 60*time.Second)
	defer timeout()

	// Initialize git operations
	gitOps := git.NewMemoryOperations(s.config.GitHub.Token)

	// Clone repository into memory
	memRepo, err := gitOps.CloneRepository(ctx, repo.CloneURL, repo.FullName)
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get all files from the repository
	files, err := memRepo.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list repository files: %w", err)
	}

	s.logger.Info("Repository clone successful", "repo", repo.FullName, "totalFiles", len(files))
	
	result := make(map[string]string)

	for _, file := range files {
		// Check if file should be excluded
		excluded := false
		for _, pattern := range excludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(file.Path)); matched {
				excluded = true
				break
			}
			if matched, _ := filepath.Match(pattern, file.Path); matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// Check if file matches include patterns
		matched := len(includePatterns) == 0 // If no include patterns, include all
		for _, pattern := range includePatterns {
			// First try to match against the full path
			if m, _ := filepath.Match(pattern, file.Path); m {
				matched = true
				break
			}
			// Then try to match against just the filename
			if m, _ := filepath.Match(pattern, filepath.Base(file.Path)); m {
				matched = true
				break
			}
			// Also handle patterns like "*.md" to match any .md file in any directory
			if strings.HasPrefix(pattern, "*.") {
				ext := strings.TrimPrefix(pattern, "*")
				if strings.HasSuffix(file.Path, ext) {
					matched = true
					break
				}
			}
		}

		if matched {
			// Skip binary files (simple check for null bytes)
			content := string(file.Content)
			if strings.Contains(content, "\x00") {
				s.logger.Debug("Skipping binary file", "fileName", file.Path)
				continue
			}

			s.logger.Info("Including file for processing", "fileName", file.Path, "contentLength", len(content))
			result[file.Path] = content
		} else {
			s.logger.Debug("File does not match patterns", "fileName", file.Path, "includePatterns", includePatterns)
		}
	}

	s.logger.Info("Found repository files for processing", "count", len(result), "repo", repo.FullName)
	return result, nil
}

// Migration methods
func (s *Service) MigrateRepository(config MigrationConfig) (*MigrationResult, error) {
	steps := []MigrationStep{
		{Name: "validate", Description: "Validate configuration", Status: "pending", Progress: 0},
		{Name: "create_github_repo", Description: "Create GitHub repository", Status: "pending", Progress: 0},
		{Name: "clone_bitbucket", Description: "Clone Bitbucket repository", Status: "pending", Progress: 0},
		{Name: "add_github_remote", Description: "Add GitHub remote", Status: "pending", Progress: 0},
		{Name: "push_to_github", Description: "Push to GitHub", Status: "pending", Progress: 0},
		{Name: "rename_default_branch", Description: "Rename default branch to main", Status: "pending", Progress: 0},
		{Name: "add_teams", Description: "Add teams to repository", Status: "pending", Progress: 0},
		{Name: "add_webhook", Description: "Add webhook", Status: "pending", Progress: 0},
		{Name: "cleanup", Description: "Cleanup temporary files", Status: "pending", Progress: 0},
	}

	result := &MigrationResult{
		Success: true,
		Message: "Migration started",
		Steps:   steps,
	}

	if config.DryRun {
		// Simulate all steps as completed for dry run
		for i := range result.Steps {
			result.Steps[i].Status = "completed"
			result.Steps[i].Progress = 100
			result.Steps[i].Message = "Would execute in real run"
		}
		result.Message = "Dry run completed - all steps would execute successfully"
		return result, nil
	}

	// Execute actual migration
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	err := s.executeMigration(ctx, config, result)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("Migration failed: %v", err)
		// Mark current step as failed
		for i := range result.Steps {
			if result.Steps[i].Status == "running" {
				result.Steps[i].Status = "failed"
				result.Steps[i].Message = err.Error()
				break
			}
		}
	} else {
		result.Success = true
		result.Message = "Migration completed successfully"
	}

	return result, nil
}

func (s *Service) executeMigration(ctx context.Context, config MigrationConfig, result *MigrationResult) error {
	// Step 1: Validate
	s.updateStepStatus(result, 0, "running", "Validating configuration...", 50)
	if err := s.ValidateMigrationConfig(config); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	s.updateStepStatus(result, 0, "completed", "Configuration validated", 100)

	// Extract repository name from Bitbucket URL
	repoName := s.extractRepositoryName(config.SourceBitbucketURL)
	if config.TargetRepositoryName != "" {
		repoName = config.TargetRepositoryName
	}

	// Step 2: Create GitHub repository
	s.updateStepStatus(result, 1, "running", "Creating GitHub repository...", 25)
	githubRepo, err := s.createGitHubRepository(ctx, config.TargetGitHubOrg, repoName)
	if err != nil {
		return fmt.Errorf("failed to create GitHub repository: %w", err)
	}
	result.GitHubRepoURL = fmt.Sprintf("https://%s/%s", s.extractGitHubHost(), githubRepo.GetFullName())
	s.updateStepStatus(result, 1, "completed", fmt.Sprintf("Repository created: %s", githubRepo.GetName()), 100)

	// Step 3: Clone Bitbucket repository
	s.updateStepStatus(result, 2, "running", "Cloning Bitbucket repository...", 25)
	tempDir := filepath.Join(os.TempDir(), "migration-"+repoName)
	defer os.RemoveAll(tempDir)

	if err := s.cloneBitbucketRepository(ctx, config.SourceBitbucketURL, tempDir); err != nil {
		return fmt.Errorf("failed to clone Bitbucket repository: %w", err)
	}
	s.updateStepStatus(result, 2, "completed", "Repository cloned successfully", 100)

	// Step 4: Add GitHub remote
	s.updateStepStatus(result, 3, "running", "Adding GitHub remote...", 50)
	githubRemoteURL := fmt.Sprintf("git@%s:%s/%s.git", s.extractGitHubHost(), config.TargetGitHubOrg, repoName)
	if err := s.addGitHubRemote(tempDir, githubRemoteURL); err != nil {
		return fmt.Errorf("failed to add GitHub remote: %w", err)
	}
	s.updateStepStatus(result, 3, "completed", "GitHub remote added", 100)

	// Step 5: Push to GitHub
	s.updateStepStatus(result, 4, "running", "Pushing to GitHub...", 25)
	if err := s.pushToGitHub(ctx, tempDir); err != nil {
		return fmt.Errorf("failed to push to GitHub: %w", err)
	}
	s.updateStepStatus(result, 4, "completed", "Successfully pushed to GitHub", 100)

	// Step 6: Rename default branch
	s.updateStepStatus(result, 5, "running", "Renaming default branch to main...", 25)
	if err := s.renameDefaultBranch(ctx, config.TargetGitHubOrg, repoName, tempDir); err != nil {
		return fmt.Errorf("failed to rename default branch: %w", err)
	}
	s.updateStepStatus(result, 5, "completed", "Default branch renamed to main", 100)

	// Step 7: Add teams
	s.updateStepStatus(result, 6, "running", "Adding teams to repository...", 25)
	addedTeams, err := s.addTeamsToRepository(ctx, config.TargetGitHubOrg, repoName, config.Teams)
	if err != nil {
		return fmt.Errorf("failed to add teams: %w", err)
	}
	result.CreatedTeams = addedTeams
	s.updateStepStatus(result, 6, "completed", fmt.Sprintf("Added %d teams", len(addedTeams)), 100)

	// Step 8: Add webhook
	s.updateStepStatus(result, 7, "running", "Adding webhook...", 25)
	if config.WebhookURL != "" {
		webhookURL, err := s.addWebhook(ctx, config.TargetGitHubOrg, repoName, config.WebhookURL)
		if err != nil {
			return fmt.Errorf("failed to add webhook: %w", err)
		}
		result.CreatedWebhooks = []string{webhookURL}
		s.updateStepStatus(result, 7, "completed", "Webhook added successfully", 100)
	} else {
		s.updateStepStatus(result, 7, "completed", "No webhook configured", 100)
	}

	// Step 9: Cleanup
	s.updateStepStatus(result, 8, "running", "Cleaning up temporary files...", 50)
	// Cleanup is handled by defer statement
	s.updateStepStatus(result, 8, "completed", "Cleanup completed", 100)

	return nil
}

func (s *Service) updateStepStatus(result *MigrationResult, stepIndex int, status, message string, progress int) {
	if stepIndex < len(result.Steps) {
		result.Steps[stepIndex].Status = status
		result.Steps[stepIndex].Message = message
		result.Steps[stepIndex].Progress = progress
	}
}

func (s *Service) extractRepositoryName(bitbucketURL string) string {
	// Extract repo name from URL like ssh://git@bitbucket.server:2222/project/repo.git
	parts := strings.Split(bitbucketURL, "/")
	if len(parts) > 0 {
		repoName := parts[len(parts)-1]
		return strings.TrimSuffix(repoName, ".git")
	}
	return "migrated-repo"
}

func (s *Service) extractGitHubHost() string {
	// Extract hostname from GitHub base URL
	baseURL := s.config.GitHub.BaseURL
	if baseURL == "" || baseURL == "https://api.github.com" {
		return "github.com"
	}
	
	u, err := url.Parse(baseURL)
	if err != nil {
		return "github.com"
	}
	
	// Convert API URL to Git SSH host
	host := u.Host
	if strings.HasPrefix(host, "api.") {
		host = strings.TrimPrefix(host, "api.")
	}
	
	return host
}

func (s *Service) createGitHubRepository(ctx context.Context, org, repoName string) (*gogithub.Repository, error) {
	repo, err := s.github.CreateRepository(ctx, &github.CreateRepositoryOptions{
		Name:         repoName,
		Organization: org,
		Private:      false, // Can be configurable
		Description:  "Repository migrated from Bitbucket Server",
	})
	if err != nil {
		return nil, err
	}
	
	s.logger.Info("GitHub repository created", "repo", repo.FullName, "url", fmt.Sprintf("https://%s/%s", s.extractGitHubHost(), repo.FullName))
	
	// Return our internal Repository type converted to gogithub.Repository for compatibility
	return &gogithub.Repository{
		ID:       &repo.ID,
		Name:     &repo.Name,
		FullName: &repo.FullName,
		CloneURL: &repo.CloneURL,
		Private:  &repo.Private,
	}, nil
}

func (s *Service) cloneBitbucketRepository(ctx context.Context, bitbucketURL, tempDir string) error {
	// Use existing git operations instead of memory operations for mirror clone
	gitOps, err := git.NewOperationsWithToken(s.config.Bitbucket.Password)
	if err != nil {
		return err
	}
	
	// Convert SSH URL to HTTPS with authentication for Bitbucket
	authURL := s.addBitbucketAuth(bitbucketURL)
	
	// Clone with --mirror option using exec
	err = gitOps.CloneRepository(authURL, tempDir)
	if err != nil {
		return err
	}
	
	s.logger.Info("Bitbucket repository cloned", "url", bitbucketURL, "dir", tempDir)
	return nil
}

func (s *Service) addBitbucketAuth(repoURL string) string {
	// Add Bitbucket credentials to URL if needed
	if s.config.Bitbucket.Username != "" && s.config.Bitbucket.Password != "" {
		// Convert SSH URL to HTTPS with credentials
		if strings.HasPrefix(repoURL, "ssh://") {
			// Extract the path from SSH URL: ssh://git@bitbucket.server:2222/project/repo.git
			parts := strings.Split(repoURL, "/")
			if len(parts) >= 4 {
				host := strings.Split(parts[2], ":")[0] // Remove port if present
				host = strings.TrimPrefix(host, "git@")
				path := strings.Join(parts[3:], "/")
				return fmt.Sprintf("https://%s:%s@%s/%s", 
					s.config.Bitbucket.Username, 
					s.config.Bitbucket.Password, 
					host, 
					path)
			}
		}
	}
	return repoURL
}

func (s *Service) addGitHubRemote(tempDir, githubURL string) error {
	// Use exec command to add remote
	cmd := exec.Command("git", "remote", "add", "github", githubURL)
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add GitHub remote: %w, output: %s", err, string(output))
	}
	
	return nil
}

func (s *Service) pushToGitHub(ctx context.Context, tempDir string) error {
	// Use git push --mirror to push all branches and tags
	cmd := exec.Command("git", "push", "--mirror", "github")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push mirror to GitHub: %w, output: %s", err, string(output))
	}
	
	s.logger.Info("Successfully pushed mirror to GitHub")
	return nil
}

func (s *Service) renameDefaultBranch(ctx context.Context, org, repo, tempDir string) error {
	// Check if master branch exists locally
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/master")
	cmd.Dir = tempDir
	err := cmd.Run()
	
	if err != nil {
		s.logger.Info("Master branch not found, skipping rename")
		return nil
	}
	
	// Rename master to main locally
	cmd = exec.Command("git", "branch", "-m", "master", "main")
	cmd.Dir = tempDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to rename master to main: %w, output: %s", err, string(output))
	}
	
	// Push new main branch to GitHub
	cmd = exec.Command("git", "push", "github", "main")
	cmd.Dir = tempDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push main branch: %w, output: %s", err, string(output))
	}
	
	// Update default branch on GitHub using API
	_, err = s.github.UpdateRepository(ctx, org, repo, &github.UpdateRepositoryOptions{
		DefaultBranch: "main",
	})
	if err != nil {
		return fmt.Errorf("failed to update default branch: %w", err)
	}
	
	// Delete old master branch from GitHub
	cmd = exec.Command("git", "push", "github", "--delete", "master")
	cmd.Dir = tempDir
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Warn("Failed to delete remote master branch", "error", err, "output", string(output))
		// Non-fatal error
	}
	
	s.logger.Info("Default branch renamed from master to main")
	return nil
}

func (s *Service) addTeamsToRepository(ctx context.Context, org, repo string, teams map[string]string) ([]string, error) {
	var addedTeams []string
	
	for teamName, permission := range teams {
		if teamName == "" {
			continue
		}
		
		err := s.github.AddTeamToRepository(ctx, org, teamName, repo, permission)
		if err != nil {
			s.logger.Warn("Failed to add team to repository", "team", teamName, "error", err)
			continue // Continue with other teams
		}
		
		addedTeams = append(addedTeams, teamName)
		s.logger.Info("Team added to repository", "team", teamName, "permission", permission)
	}
	
	return addedTeams, nil
}

func (s *Service) addWebhook(ctx context.Context, org, repo, webhookURL string) (string, error) {
	webhook, err := s.github.CreateWebhook(ctx, org, repo, &github.CreateWebhookOptions{
		URL:         webhookURL,
		ContentType: "json",
		Events:      []string{"push"},
		Active:      true,
	})
	if err != nil {
		return "", err
	}
	
	s.logger.Info("Webhook added to repository", "url", webhookURL, "webhook_id", webhook.GetID())
	return webhook.GetURL(), nil
}

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

	// Validate Bitbucket client
	if s.bitbucket == nil {
		return fmt.Errorf("Bitbucket client not initialized - please configure Bitbucket access first")
	}

	// Validate GitHub client
	if s.github == nil {
		return fmt.Errorf("GitHub client not initialized - please configure GitHub access first")
	}

	return nil
}
