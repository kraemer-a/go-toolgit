package gui

import (
	"context"
	"fmt"
	"io/fs"
	"os"
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
)

type Service struct {
	config    *config.Config
	logger    *utils.Logger
	github    *github.Client
	bitbucket *bitbucket.Client
}

type ConfigData struct {
	Provider         string `json:"provider"` // "github" or "bitbucket"
	GitHubURL        string `json:"github_url"`
	Token            string `json:"token"`
	Organization     string `json:"organization"`
	Team             string `json:"team"`
	BitbucketURL     string `json:"bitbucket_url"`
	BitbucketUser    string `json:"bitbucket_username"`
	BitbucketPass    string `json:"bitbucket_password"`
	BitbucketProject string `json:"bitbucket_project"`
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

	s.logger.Info("Configuration updated successfully", "provider", s.config.Provider)
	return nil
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
		if !repo.Selected {
			continue
		}

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
				if !rule.CaseSensitive {
					if strings.Contains(strings.ToLower(modifiedContent), strings.ToLower(searchStr)) {
						s.logger.Debug("Case-insensitive match found", "fileName", fileName, "searchStr", searchStr)
						// Case-insensitive replacement
						re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(searchStr))
						modifiedContent = re.ReplaceAllString(modifiedContent, rule.Replacement)
						hasChanges = true
					} else {
						s.logger.Debug("No case-insensitive match", "fileName", fileName, "searchStr", searchStr)
					}
				} else {
					if strings.Contains(modifiedContent, searchStr) {
						s.logger.Debug("Case-sensitive match found", "fileName", fileName, "searchStr", searchStr)
						modifiedContent = strings.ReplaceAll(modifiedContent, searchStr, rule.Replacement)
						hasChanges = true
					} else {
						s.logger.Debug("No case-sensitive match", "fileName", fileName, "searchStr", searchStr)
					}
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
				if !rule.CaseSensitive {
					if strings.Contains(strings.ToLower(modifiedContent), strings.ToLower(searchStr)) {
						// Case-insensitive replacement
						re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(searchStr))
						matches := re.FindAllString(modifiedContent, -1)
						modifiedContent = re.ReplaceAllString(modifiedContent, rule.Replacement)
						hasChanges = true
						fileReplacements += len(matches)
					}
				} else {
					count := strings.Count(modifiedContent, searchStr)
					if count > 0 {
						modifiedContent = strings.ReplaceAll(modifiedContent, searchStr, rule.Replacement)
						hasChanges = true
						fileReplacements += count
					}
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
	s.logger.Debug("Getting repository files via git", "repo", repo.FullName)

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
			// Skip binary files (simple check for null bytes)
			content := string(file.Content)
			if strings.Contains(content, "\x00") {
				s.logger.Debug("Skipping binary file", "fileName", file.Path)
				continue
			}

			result[file.Path] = content
		}
	}

	s.logger.Debug("Found repository files for processing", "count", len(result), "repo", repo.FullName)
	return result, nil
}
