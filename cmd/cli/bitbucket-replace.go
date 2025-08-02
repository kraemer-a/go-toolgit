package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/bitbucket"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/git"
	"go-toolgit/internal/core/processor"
	"go-toolgit/internal/core/utils"
)

var bitbucketReplaceCmd = &cobra.Command{
	Use:   "bitbucket-replace",
	Short: "Replace strings across Bitbucket repositories",
	Long: `Replace strings across all repositories in a Bitbucket project.
This command will:
1. List all repositories in the specified project
2. Clone each repository
3. Apply string replacements
4. Create pull requests with the changes

Example:
  go-toolgit bitbucket-replace --bitbucket-url https://bitbucket.company.com --bitbucket-username user --bitbucket-password token --bitbucket-project PROJ --replacements "oldString=newString,foo=bar"`,
	RunE: runBitbucketReplace,
}

var (
	bitbucketReplacements    []string
	bitbucketIncludePatterns []string
	bitbucketExcludePatterns []string
	bitbucketDryRun          bool
	bitbucketPrTitle         string
	bitbucketPrBody          string
	bitbucketBranchPrefix    string
	bitbucketMaxWorkers      int
)

func init() {
	rootCmd.AddCommand(bitbucketReplaceCmd)

	bitbucketReplaceCmd.Flags().StringSliceVar(&bitbucketReplacements, "replacements", []string{}, "String replacements in format 'original=replacement' (required)")
	bitbucketReplaceCmd.Flags().StringSliceVar(&bitbucketIncludePatterns, "include", []string{}, "File patterns to include (e.g., '*.go,*.java')")
	bitbucketReplaceCmd.Flags().StringSliceVar(&bitbucketExcludePatterns, "exclude", []string{}, "File patterns to exclude")
	bitbucketReplaceCmd.Flags().BoolVar(&bitbucketDryRun, "dry-run", false, "Preview changes without executing")
	bitbucketReplaceCmd.Flags().StringVar(&bitbucketPrTitle, "pr-title", "", "Pull request title template")
	bitbucketReplaceCmd.Flags().StringVar(&bitbucketPrBody, "pr-body", "", "Pull request body template")
	bitbucketReplaceCmd.Flags().StringVar(&bitbucketBranchPrefix, "branch-prefix", "", "Prefix for created branches")
	bitbucketReplaceCmd.Flags().IntVar(&bitbucketMaxWorkers, "max-workers", 4, "Maximum number of concurrent workers")

	bitbucketReplaceCmd.MarkFlagRequired("replacements")
}

func runBitbucketReplace(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	ctx := context.Background()

	// Setup phase with spinner
	setupSpinner, err := utils.NewSpinner("Initializing Bitbucket replacement operation")
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

	// Force provider to bitbucket for this command
	cfg.Provider = "bitbucket"

	if err := cfg.Validate(); err != nil {
		setupSpinner.StopWithFailure("Configuration validation failed")
		return utils.NewValidationError("configuration validation failed", err)
	}

	rules, err := parseReplacements(bitbucketReplacements)
	if err != nil {
		setupSpinner.StopWithFailure("Invalid replacement rules")
		return utils.NewValidationError("invalid replacement rules", err)
	}

	setupSpinner.StopWithSuccess("Configuration and rules validated")

	if IsDebugMode() {
		logger.Info("Starting Bitbucket string replacement operation",
			"project", cfg.Bitbucket.Project,
			"dry_run", bitbucketDryRun,
			"rules_count", len(rules))
	}

	// Bitbucket connection phase with spinner
	bitbucketSpinner, err := utils.NewSpinner("Connecting to Bitbucket Server")
	if err != nil {
		return err
	}

	if err := bitbucketSpinner.Start(); err != nil {
		return err
	}

	bitbucketClient, err := bitbucket.NewClient(&bitbucket.Config{
		BaseURL:    cfg.Bitbucket.BaseURL,
		Username:   cfg.Bitbucket.Username,
		Password:   cfg.Bitbucket.Password,
		Timeout:    cfg.Bitbucket.Timeout,
		MaxRetries: cfg.Bitbucket.MaxRetries,
	})
	if err != nil {
		bitbucketSpinner.StopWithFailure("Failed to create Bitbucket client")
		return utils.NewAuthError("failed to create Bitbucket client", err)
	}

	bitbucketSpinner.UpdateMessage("Validating Bitbucket access")

	if err := bitbucketClient.ValidateAccess(ctx); err != nil {
		bitbucketSpinner.StopWithFailure("Bitbucket access validation failed")
		return utils.NewAuthError("failed to validate Bitbucket access", err)
	}

	bitbucketSpinner.UpdateMessage("Listing project repositories")

	repositories, err := bitbucketClient.ListProjectRepositories(ctx, cfg.Bitbucket.Project)
	if err != nil {
		bitbucketSpinner.StopWithFailure("Failed to list project repositories")
		return utils.NewNetworkError("failed to list project repositories", err)
	}

	bitbucketSpinner.StopWithSuccess(fmt.Sprintf("Found %d repositories to process", len(repositories)))
	if IsDebugMode() {
		logger.Info("Found repositories", "count", len(repositories))
	}

	gitOps, err := git.NewOperationsWithToken(cfg.Bitbucket.Password)
	if err != nil {
		return utils.NewGitError("failed to initialize git operations", err)
	}

	bitbucketIncludePatterns = mergePatterns(bitbucketIncludePatterns, cfg.Processing.IncludePatterns)
	bitbucketExcludePatterns = mergePatterns(bitbucketExcludePatterns, cfg.Processing.ExcludePatterns)

	engine, err := processor.NewReplacementEngine(rules, bitbucketIncludePatterns, bitbucketExcludePatterns)
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

		if err := processBitbucketRepository(ctx, repo, engine, gitOps, bitbucketClient, cfg, repoLogger); err != nil {
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
		logger.Info("Bitbucket replacement operation completed",
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

func processBitbucketRepository(ctx context.Context, repo *bitbucket.Repository, engine *processor.ReplacementEngine,
	gitOps *git.Operations, bitbucketClient *bitbucket.Client, cfg *config.Config, logger *utils.Logger) error {

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
	changes, err := engine.ProcessDirectory(tempDir, bitbucketDryRun)
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

	if bitbucketDryRun {
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

	branchName := gitOps.GenerateBranchName(getBitbucketBranchPrefix(cfg))
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

	prOptions := &bitbucket.PullRequestOptions{
		Title:       getBitbucketPRTitle(cfg),
		Description: getBitbucketPRBody(cfg, changes),
		FromRef: bitbucket.Ref{
			ID: "refs/heads/" + branchName,
			Repository: bitbucket.Repository{
				Name:    repo.Name,
				Project: repo.Project,
			},
		},
		ToRef: bitbucket.Ref{
			ID: "refs/heads/master", // TODO: detect default branch
			Repository: bitbucket.Repository{
				Name:    repo.Name,
				Project: repo.Project,
			},
		},
	}

	if IsDebugMode() {
		logger.Info("Creating pull request")
	}
	pr, err := bitbucketClient.CreatePullRequest(ctx, repo.Project, repo.Name, prOptions)
	if err != nil {
		// If PR creation fails with master, try main
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "invalid") {
			if IsDebugMode() {
				logger.Info("Branch 'master' not found, trying 'main'")
			}
			prOptions.ToRef.ID = "refs/heads/main"
			pr, err = bitbucketClient.CreatePullRequest(ctx, repo.Project, repo.Name, prOptions)
		}

		if err != nil {
			return utils.NewNetworkError(fmt.Sprintf("failed to create pull request against branch '%s'", strings.TrimPrefix(prOptions.ToRef.ID, "refs/heads/")), err)
		}
	}

	if IsDebugMode() {
		logger.Info("Pull request created", "pr_url", pr.GetHTMLURL())
	}
	return nil
}

func getBitbucketBranchPrefix(cfg *config.Config) string {
	if bitbucketBranchPrefix != "" {
		return bitbucketBranchPrefix
	}
	return cfg.PullRequest.BranchPrefix
}

func getBitbucketPRTitle(cfg *config.Config) string {
	if bitbucketPrTitle != "" {
		return bitbucketPrTitle
	}
	return cfg.PullRequest.TitleTemplate
}

func getBitbucketPRBody(cfg *config.Config, changes []*processor.FileChange) string {
	if bitbucketPrBody != "" {
		return bitbucketPrBody
	}

	body := cfg.PullRequest.BodyTemplate + "\n\n## Changes Summary\n"
	for _, change := range changes {
		body += fmt.Sprintf("- Modified %s (%d replacements)\n", change.FilePath, change.Replacements)
	}

	return body
}
