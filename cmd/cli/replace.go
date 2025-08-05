package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/git"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/processor"
	"go-toolgit/internal/core/utils"
)

var replaceCmd = &cobra.Command{
	Use:   "replace",
	Short: "Replace strings across repositories",
	Long: `Replace strings across all repositories accessible to the specified team.
This command will:
1. List all repositories for the team
2. Clone each repository
3. Apply string replacements
4. Create pull requests with the changes

Example:
  go-toolgit replace --org myorg --team myteam --replacements "oldString=newString,foo=bar"`,
	RunE: runReplace,
}

var (
	replacements    []string
	includePatterns []string
	excludePatterns []string
	dryRun          bool
	prTitle         string
	prBody          string
	branchPrefix    string
	maxWorkers      int
)

func init() {
	rootCmd.AddCommand(replaceCmd)

	replaceCmd.Flags().StringSliceVar(&replacements, "replacements", []string{}, "String replacements in format 'original=replacement' (required)")
	replaceCmd.Flags().StringSliceVar(&includePatterns, "include", []string{}, "File patterns to include (e.g., '*.go,*.java')")
	replaceCmd.Flags().StringSliceVar(&excludePatterns, "exclude", []string{}, "File patterns to exclude")
	replaceCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without executing")
	replaceCmd.Flags().StringVar(&prTitle, "pr-title", "", "Pull request title template")
	replaceCmd.Flags().StringVar(&prBody, "pr-body", "", "Pull request body template")
	replaceCmd.Flags().StringVar(&branchPrefix, "branch-prefix", "", "Prefix for created branches")
	replaceCmd.Flags().IntVar(&maxWorkers, "max-workers", 4, "Maximum number of concurrent workers")

	replaceCmd.MarkFlagRequired("replacements")
}

func runReplace(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	ctx := context.Background()

	// Setup phase with spinner
	setupSpinner, err := utils.NewSpinner("Initializing replacement operation")
	if err != nil {
		return err
	}

	if err := setupSpinner.Start(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		setupSpinner.StopWithFailure("Failed to load configuration")
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		setupSpinner.StopWithFailure("Configuration validation failed")
		return utils.NewValidationError("configuration validation failed", err)
	}

	rules, err := parseReplacements(replacements)
	if err != nil {
		setupSpinner.StopWithFailure("Invalid replacement rules")
		return utils.NewValidationError("invalid replacement rules", err)
	}

	setupSpinner.StopWithSuccess("Configuration and rules validated")

	if IsDebugMode() {
		logger.Info("Starting string replacement operation",
			"org", cfg.GitHub.Org,
			"team", cfg.GitHub.Team,
			"dry_run", dryRun,
			"rules_count", len(rules))
	}

	// GitHub connection phase with spinner
	githubSpinner, err := utils.NewSpinner("Connecting to GitHub API")
	if err != nil {
		return err
	}

	if err := githubSpinner.Start(); err != nil {
		return err
	}

	githubClient, err := github.NewClient(&github.Config{
		BaseURL:      cfg.GitHub.BaseURL,
		Token:        cfg.GitHub.Token,
		Timeout:      cfg.GitHub.Timeout,
		MaxRetries:   cfg.GitHub.MaxRetries,
		WaitForReset: cfg.GitHub.WaitForRateLimit,
	})
	if err != nil {
		githubSpinner.StopWithFailure("Failed to create GitHub client")
		return utils.NewAuthError("failed to create GitHub client", err)
	}

	githubSpinner.UpdateMessage("Validating GitHub access")

	if err := githubClient.ValidateAccess(ctx, cfg.GitHub.Org, cfg.GitHub.Team); err != nil {
		githubSpinner.StopWithFailure("GitHub access validation failed")
		return utils.NewAuthError("failed to validate GitHub access", err)
	}

	githubSpinner.UpdateMessage("Getting team information")

	team, err := githubClient.GetTeam(ctx, cfg.GitHub.Org, cfg.GitHub.Team)
	if err != nil {
		githubSpinner.StopWithFailure("Failed to get team information")
		return utils.NewNetworkError("failed to get team information", err)
	}

	githubSpinner.UpdateMessage("Listing team repositories")

	repositories, err := githubClient.ListTeamRepositories(ctx, team)
	if err != nil {
		githubSpinner.StopWithFailure("Failed to list team repositories")
		return utils.NewNetworkError("failed to list team repositories", err)
	}

	githubSpinner.StopWithSuccess(fmt.Sprintf("Found %d repositories to process", len(repositories)))
	if IsDebugMode() {
		logger.Info("Found repositories", "count", len(repositories))
	}

	gitOps, err := git.NewOperationsWithToken(cfg.GitHub.Token)
	if err != nil {
		return utils.NewGitError("failed to initialize git operations", err)
	}

	includePatterns = mergePatterns(includePatterns, cfg.Processing.IncludePatterns)
	excludePatterns = mergePatterns(excludePatterns, cfg.Processing.ExcludePatterns)

	engine, err := processor.NewReplacementEngine(rules, includePatterns, excludePatterns)
	if err != nil {
		return utils.NewProcessingError("failed to create replacement engine", err)
	}

	successCount := 0
	errorCount := 0

	// Processing phase with spinner
	var processingSpinner *utils.Spinner
	if len(repositories) > 0 {
		processingSpinner, err = utils.NewSpinner(fmt.Sprintf("Processing %d repositories", len(repositories)))
		if err != nil {
			return err
		}

		if err := processingSpinner.Start(); err != nil {
			return err
		}
	}

	for i, repo := range repositories {
		if processingSpinner != nil {
			processingSpinner.UpdateMessage(fmt.Sprintf("Processing repository %d/%d: %s", i+1, len(repositories), repo.Name))
		}

		repoLogger := logger.WithRepository(repo.FullName)
		if IsDebugMode() {
			repoLogger.Info("Processing repository")
		}

		if err := processRepository(ctx, repo, engine, gitOps, githubClient, cfg, repoLogger); err != nil {
			if IsDebugMode() {
				repoLogger.Error("Failed to process repository", "error", err)
			}
			errorCount++
		} else {
			successCount++
		}
	}

	if processingSpinner != nil {
		processingSpinner.StopWithSuccess(fmt.Sprintf("Processed %d repositories (%d successful, %d failed)", len(repositories), successCount, errorCount))
	}

	stats := engine.GetStats()
	if IsDebugMode() {
		logger.Info("Replacement operation completed",
			"successful_repos", successCount,
			"failed_repos", errorCount,
			"files_processed", stats.FilesProcessed,
			"files_modified", stats.FilesModified,
			"total_replacements", stats.Replacements)

		if len(stats.Errors) > 0 {
			logger.Warn("Encountered errors during processing", "error_count", len(stats.Errors))
			for _, err := range stats.Errors {
				logger.Error("Processing error", "error", err)
			}
		}
	}

	return nil
}

func processRepository(ctx context.Context, repo *github.Repository, engine *processor.ReplacementEngine,
	gitOps *git.Operations, githubClient *github.Client, cfg *config.Config, logger *utils.Logger) error {

	tempDir := filepath.Join(os.TempDir(), "go-toolgit", repo.Name)
	defer gitOps.CleanupRepository(tempDir)

	if IsDebugMode() {
		logger.Info("Cloning repository", "temp_dir", tempDir)
	}
	if err := gitOps.CloneRepository(repo.CloneURL, tempDir); err != nil {
		return utils.NewGitError("failed to clone repository", err)
	}

	if IsDebugMode() {
		logger.Info("Processing files")
	}
	changes, err := engine.ProcessDirectory(tempDir, dryRun)
	if err != nil {
		return utils.NewProcessingError("failed to process files", err)
	}

	if len(changes) == 0 {
		if IsDebugMode() {
			logger.Info("No changes required")
		}
		return nil
	}

	if IsDebugMode() {
		logger.Info("Found changes", "modified_files", len(changes))
	}

	if dryRun {
		if IsDebugMode() {
			logger.Info("Dry run mode - changes preview:")
			for _, change := range changes {
				logger.Info("Would modify file",
					"file", change.FilePath,
					"replacements", change.Replacements)
			}
		}

		// Show detailed string changes if available
		for _, change := range changes {
			if len(change.StringChanges) > 0 {
				fmt.Printf("\n📁 File: %s\n", change.FilePath)
				fmt.Printf("   %d replacement(s) found:\n\n", change.Replacements)

				for i, stringChange := range change.StringChanges {
					fmt.Printf("   %d. Line %d:\n", i+1, stringChange.LineNumber)
					fmt.Printf("      - Original: %q\n", stringChange.Original)
					fmt.Printf("      + Replace:  %q\n", stringChange.Replacement)
					fmt.Printf("      Context:    %s\n", stringChange.Context)
					if i < len(change.StringChanges)-1 {
						fmt.Println()
					}
				}
				fmt.Println()
			}
		}
		return nil
	}

	hasChanges, err := gitOps.HasChanges(tempDir)
	if err != nil {
		return utils.NewGitError("failed to check for changes", err)
	}

	if !hasChanges {
		if IsDebugMode() {
			logger.Info("No Git changes detected")
		}
		return nil
	}

	branchName := gitOps.GenerateBranchName(getBranchPrefix(cfg))
	if IsDebugMode() {
		logger.Info("Creating branch", "branch", branchName)
	}

	if err := gitOps.CreateBranch(tempDir, branchName); err != nil {
		return utils.NewGitError("failed to create branch", err)
	}

	if err := gitOps.AddAllChanges(tempDir); err != nil {
		return utils.NewGitError("failed to add changes", err)
	}

	commitMessage := generateCommitMessage(cfg, changes)
	if err := gitOps.Commit(tempDir, git.CommitOptions{
		Message: commitMessage,
		Author:  "GitHub Replace Tool",
		Email:   "go-toolgit@automated.tool",
	}); err != nil {
		return utils.NewGitError("failed to commit changes", err)
	}

	if IsDebugMode() {
		logger.Info("Pushing branch")
	}
	if err := gitOps.Push(tempDir, branchName); err != nil {
		return utils.NewGitError("failed to push branch", err)
	}

	parts := strings.Split(repo.FullName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository full name format: %s", repo.FullName)
	}

	prOptions := &github.PullRequestOptions{
		Title: getPRTitle(cfg),
		Head:  branchName,
		Base:  "main",
		Body:  getPRBody(cfg, changes),
	}

	if IsDebugMode() {
		logger.Info("Creating pull request")
	}
	pr, err := githubClient.CreatePullRequest(ctx, parts[0], parts[1], prOptions)
	if err != nil {
		// If PR creation fails with main, try master
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "invalid") {
			if IsDebugMode() {
				logger.Info("Branch 'main' not found, trying 'master'")
			}
			prOptions.Base = "master"
			pr, err = githubClient.CreatePullRequest(ctx, parts[0], parts[1], prOptions)
		}

		if err != nil {
			return utils.NewNetworkError(fmt.Sprintf("failed to create pull request against branch '%s'", prOptions.Base), err)
		}
	}

	if IsDebugMode() {
		logger.Info("Pull request created", "pr_url", pr.GetHTMLURL())
	}
	return nil
}

func parseReplacements(replacements []string) ([]processor.ReplacementRule, error) {
	var rules []processor.ReplacementRule

	for _, replacement := range replacements {
		parts := strings.SplitN(replacement, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid replacement format: %s (expected 'original=replacement')", replacement)
		}

		rule := processor.ReplacementRule{
			Original:      parts[0],
			Replacement:   parts[1],
			CaseSensitive: true,
		}

		rules = append(rules, rule)
	}

	return rules, nil
}

func mergePatterns(cmdPatterns, configPatterns []string) []string {
	if len(cmdPatterns) > 0 {
		return cmdPatterns
	}
	return configPatterns
}

func getBranchPrefix(cfg *config.Config) string {
	if branchPrefix != "" {
		return branchPrefix
	}
	return cfg.PullRequest.BranchPrefix
}

func getPRTitle(cfg *config.Config) string {
	if prTitle != "" {
		return prTitle
	}
	return cfg.PullRequest.TitleTemplate
}

func getPRBody(cfg *config.Config, changes []*processor.FileChange) string {
	if prBody != "" {
		return prBody
	}

	body := cfg.PullRequest.BodyTemplate + "\n\n## Changes Summary\n"
	for _, change := range changes {
		body += fmt.Sprintf("- Modified %s (%d replacements)\n", change.FilePath, change.Replacements)
	}

	return body
}

func generateCommitMessage(cfg *config.Config, changes []*processor.FileChange) string {
	totalReplacements := 0
	for _, change := range changes {
		totalReplacements += change.Replacements
	}

	return fmt.Sprintf("chore: automated string replacement\n\nApplied %d replacements across %d files",
		totalReplacements, len(changes))
}
