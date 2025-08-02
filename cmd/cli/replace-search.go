package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/git"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/processor"
	"go-toolgit/internal/core/utils"
)

var replaceSearchCmd = &cobra.Command{
	Use:   "replace-search",
	Short: "Replace strings in repositories found by search",
	Long: `Perform string replacements across repositories found through GitHub search.
This combines search functionality with replacement operations.

Examples:
  # Replace in all repositories of a user
  go-toolgit replace-search --owner gurke --replacements "oldAPI=newAPI"

  # Replace in Go repositories with high stars
  go-toolgit replace-search --language go --stars ">100" --replacements "fmt.Print=log.Info"

  # Complex search with multiple replacements
  go-toolgit replace-search --query "microservice" --language go --replacements "v1=v2,old=new"`,
	RunE: runReplaceSearch,
}

var (
	searchReplacements []string
	searchDryRun       bool
	searchMaxRepos     int
)

func init() {
	rootCmd.AddCommand(replaceSearchCmd)

	// Search criteria flags (reuse from search command)
	replaceSearchCmd.Flags().StringVar(&searchQuery, "query", "", "Search query (keywords)")
	replaceSearchCmd.Flags().StringVar(&searchOwner, "owner", "", "Repository owner (user or organization)")
	replaceSearchCmd.Flags().StringVar(&searchLanguage, "language", "", "Programming language")
	replaceSearchCmd.Flags().StringVar(&searchStars, "stars", "", "Star count (e.g., '>100', '50..100')")
	replaceSearchCmd.Flags().StringVar(&searchSize, "size", "", "Repository size in KB")
	replaceSearchCmd.Flags().BoolVar(&includeForks, "include-forks", false, "Include forked repositories")
	replaceSearchCmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include archived repositories")
	replaceSearchCmd.Flags().StringVar(&sortBy, "sort", "updated", "Sort by: stars, forks, updated")
	replaceSearchCmd.Flags().StringVar(&sortOrder, "order", "desc", "Sort order: asc, desc")

	// Replacement flags
	replaceSearchCmd.Flags().StringSliceVar(&searchReplacements, "replacements", []string{}, "String replacements in format 'original=replacement' (required)")
	replaceSearchCmd.Flags().StringSliceVar(&includePatterns, "include", []string{}, "File patterns to include")
	replaceSearchCmd.Flags().StringSliceVar(&excludePatterns, "exclude", []string{}, "File patterns to exclude")
	replaceSearchCmd.Flags().BoolVar(&searchDryRun, "dry-run", false, "Preview changes without executing")
	replaceSearchCmd.Flags().StringVar(&prTitle, "pr-title", "", "Pull request title template")
	replaceSearchCmd.Flags().StringVar(&prBody, "pr-body", "", "Pull request body template")
	replaceSearchCmd.Flags().StringVar(&branchPrefix, "branch-prefix", "", "Prefix for created branches")
	replaceSearchCmd.Flags().IntVar(&searchMaxRepos, "max-repos", 50, "Maximum number of repositories to process")
	replaceSearchCmd.Flags().IntVar(&maxWorkers, "max-workers", 4, "Maximum number of concurrent workers")

	replaceSearchCmd.MarkFlagRequired("replacements")
}

func runReplaceSearch(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	ctx := context.Background()

	// Setup phase with spinner
	setupSpinner, err := utils.NewSpinner("Initializing search and replace operation")
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

	if err := cfg.ValidateForSearch(); err != nil {
		setupSpinner.StopWithFailure("Configuration validation failed")
		return utils.NewValidationError("configuration validation failed", err)
	}

	rules, err := parseReplacements(searchReplacements)
	if err != nil {
		setupSpinner.StopWithFailure("Invalid replacement rules")
		return utils.NewValidationError("invalid replacement rules", err)
	}

	// Validate search criteria
	if searchQuery == "" && searchOwner == "" && searchLanguage == "" && searchStars == "" && searchSize == "" {
		setupSpinner.StopWithFailure("No search criteria specified")
		return fmt.Errorf("at least one search criterion must be specified (query, owner, language, stars, or size)")
	}

	setupSpinner.StopWithSuccess("Configuration and rules validated")

	if IsDebugMode() {
		logger.Info("Starting search and replace operation",
			"search_query", searchQuery,
			"search_owner", searchOwner,
			"search_language", searchLanguage,
			"dry_run", searchDryRun,
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
		BaseURL:    cfg.GitHub.BaseURL,
		Token:      cfg.GitHub.Token,
		Timeout:    cfg.GitHub.Timeout,
		MaxRetries: cfg.GitHub.MaxRetries,
	})
	if err != nil {
		githubSpinner.StopWithFailure("Failed to create GitHub client")
		return utils.NewAuthError("failed to create GitHub client", err)
	}

	githubSpinner.UpdateMessage("Searching repositories")

	// Build search options
	searchOpts := github.SearchOptions{
		Query:      searchQuery,
		Owner:      searchOwner,
		Language:   searchLanguage,
		Stars:      searchStars,
		Size:       searchSize,
		Fork:       includeForks,
		Archived:   includeArchived,
		Sort:       sortBy,
		Order:      sortOrder,
		MaxResults: searchMaxRepos,
	}

	repositories, err := githubClient.SearchRepositories(ctx, searchOpts)
	if err != nil {
		githubSpinner.StopWithFailure("Repository search failed")
		return utils.NewNetworkError("failed to search repositories", err)
	}

	if len(repositories) == 0 {
		githubSpinner.StopWithFailure("No repositories found")
		return fmt.Errorf("no repositories found matching search criteria")
	}

	githubSpinner.StopWithSuccess(fmt.Sprintf("Found %d repositories to process", len(repositories)))
	if IsDebugMode() {
		logger.Info("Found repositories", "count", len(repositories))
	}

	// Initialize in-memory git operations
	memoryGitOps := git.NewMemoryOperations(cfg.GitHub.Token)

	// Merge patterns with config
	includePatterns = mergePatterns(includePatterns, cfg.Processing.IncludePatterns)
	excludePatterns = mergePatterns(excludePatterns, cfg.Processing.ExcludePatterns)

	// Create replacement engine
	engine, err := processor.NewReplacementEngine(rules, includePatterns, excludePatterns)
	if err != nil {
		return utils.NewProcessingError("failed to create replacement engine", err)
	}

	// Create memory processor
	memoryProcessor := processor.NewMemoryProcessor(engine, memoryGitOps)

	successCount := 0
	errorCount := 0
	skippedCount := 0
	var skippedRepos []string

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

		// Parse owner and repo name from FullName
		parts := strings.Split(repo.FullName, "/")
		if len(parts) != 2 {
			if IsDebugMode() {
				repoLogger.Error("Invalid repository name format", "name", repo.FullName)
			}
			errorCount++
			continue
		}
		owner, repoName := parts[0], parts[1]

		// Process repository using in-memory git operations
		result, err := memoryProcessor.ProcessRepository(ctx, repo.CloneURL, repo.FullName, getBranchPrefix(cfg), searchDryRun)
		if err != nil {
			// Check if this is a repository access issue
			if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "403") {
				if IsDebugMode() {
					repoLogger.Warn("Skipping inaccessible repository", "reason", "repository may be private, deleted, or moved")
				}
				skippedRepos = append(skippedRepos, repo.FullName)
				skippedCount++
			} else {
				if IsDebugMode() {
					repoLogger.Error("Failed to process repository", "error", err)
				}
				errorCount++
			}
			continue
		}

		if result.Success {
			if IsDebugMode() {
				repoLogger.Info("Repository processed successfully",
					"files_changed", len(result.FilesChanged),
					"replacements", result.Replacements,
					"branch", result.Branch)
			}
			successCount++

			// Show detailed changes in dry-run mode
			if searchDryRun && len(result.FileChanges) > 0 {
				fmt.Printf("\n🔍 Repository: %s\n", repo.FullName)

				for _, change := range result.FileChanges {
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
			}

			// Create pull request if not dry run and changes were made
			if !searchDryRun && len(result.FilesChanged) > 0 {
				fmt.Printf("\n🔄 Creating pull request for %s...\n", repo.FullName)
				err := createPullRequest(ctx, githubClient, owner, repoName, result, cfg)
				if err != nil {
					fmt.Printf("❌ Failed to create pull request for %s: %v\n", repo.FullName, err)
					if IsDebugMode() {
						repoLogger.Warn("Failed to create pull request", "error", err)
					}
				} else {
					fmt.Printf("✅ Pull request created for %s (branch: %s)\n", repo.FullName, result.Branch)
					if IsDebugMode() {
						repoLogger.Info("Pull request created", "branch", result.Branch)
					}
				}
			}
		} else {
			if IsDebugMode() {
				repoLogger.Info("No changes required")
			}
			successCount++
		}
	}

	if processingSpinner != nil {
		processingSpinner.StopWithSuccess(fmt.Sprintf("Processed %d repositories (%d successful, %d failed, %d skipped)",
			len(repositories), successCount, errorCount, skippedCount))
	}

	// Display summary
	stats := engine.GetStats()
	fmt.Printf("\n🎯 Search and Replace Summary:\n")
	fmt.Printf("  Search criteria: %s\n", buildSearchSummary(searchOpts))
	fmt.Printf("  Repositories found: %d\n", len(repositories))
	fmt.Printf("  Repositories processed: %d\n", successCount+errorCount)
	fmt.Printf("  Successful: %d\n", successCount)
	fmt.Printf("  Failed: %d\n", errorCount)
	fmt.Printf("  Skipped: %d\n", skippedCount)
	fmt.Printf("  Files processed: %d\n", stats.FilesProcessed)
	fmt.Printf("  Files modified: %d\n", stats.FilesModified)
	fmt.Printf("  Total replacements: %d\n", stats.Replacements)

	if searchDryRun {
		fmt.Printf("\n💡 This was a dry run. Use --dry-run=false to apply changes.\n")
	}

	if skippedCount > 0 {
		fmt.Printf("\n⚠️  Note: %d repositories were skipped because they are not accessible.\n", skippedCount)

		// Show list of skipped repositories if not too many
		if len(skippedRepos) <= 10 {
			fmt.Printf("   Skipped repositories:\n")
			for _, repo := range skippedRepos {
				fmt.Printf("   • %s\n", repo)
			}
		} else {
			fmt.Printf("   First 10 skipped repositories:\n")
			for i, repo := range skippedRepos[:10] {
				fmt.Printf("   • %s\n", repo)
				if i == 9 {
					fmt.Printf("   ... and %d more\n", len(skippedRepos)-10)
				}
			}
		}

		fmt.Printf("\n   Possible reasons:\n")
		fmt.Printf("   • Private repositories you don't have access to (even with repo scope)\n")
		fmt.Printf("   • Repositories that were deleted after being indexed\n")
		fmt.Printf("   • Repositories that were moved or renamed\n")
		fmt.Printf("   • Organization repositories requiring specific team membership\n")
		fmt.Printf("\n   💡 Tip: Use more specific search criteria to reduce irrelevant results\n")
	}

	if IsDebugMode() {
		logger.Info("Search and replace operation completed",
			"successful_repos", successCount,
			"failed_repos", errorCount,
			"skipped_repos", skippedCount,
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

// createPullRequest creates a pull request for the changes
func createPullRequest(ctx context.Context, githubClient *github.Client, owner, repo string, result *processor.MemoryProcessResult, cfg *config.Config) error {
	// Detect the default branch
	defaultBranch, err := detectDefaultBranch(ctx, githubClient, owner, repo)
	if err != nil {
		// Fallback to common default branches
		if IsDebugMode() {
			fmt.Printf("⚠️  Could not detect default branch for %s/%s, trying common defaults\n", owner, repo)
		}
		defaultBranch = "main" // Start with main as most common
	}

	prOptions := &github.PullRequestOptions{
		Title: getPRTitle(cfg),
		Head:  result.Branch,
		Base:  defaultBranch,
		Body:  getPRBodyFromResult(cfg, result),
	}

	pr, err := githubClient.CreatePullRequest(ctx, owner, repo, prOptions)
	if err != nil {
		// If PR creation fails with main, try master
		if defaultBranch == "main" && (strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "invalid")) {
			if IsDebugMode() {
				fmt.Printf("⚠️  Branch 'main' not found, trying 'master' for %s/%s\n", owner, repo)
			}
			prOptions.Base = "master"
			pr, err = githubClient.CreatePullRequest(ctx, owner, repo, prOptions)
		}

		if err != nil {
			return fmt.Errorf("failed to create pull request against branch '%s': %w", prOptions.Base, err)
		}
	}

	if IsDebugMode() {
		fmt.Printf("✅ Created pull request: %s\n", pr.GetHTMLURL())
	}
	return nil
}

// detectDefaultBranch attempts to detect the default branch of a repository
func detectDefaultBranch(ctx context.Context, githubClient *github.Client, owner, repo string) (string, error) {
	// This would require adding a method to the github client to get repository info
	// For now, we'll implement a simple fallback strategy
	// TODO: Add GetRepository method to github client
	return "main", fmt.Errorf("default branch detection not implemented")
}

// getPRBodyFromResult generates PR body from processing result
func getPRBodyFromResult(cfg *config.Config, result *processor.MemoryProcessResult) string {
	body := cfg.PullRequest.BodyTemplate
	if body == "" {
		body = "Automated string replacement performed by go-toolgit tool."
	}

	body += fmt.Sprintf("\n\n## Changes Summary\n- Files modified: %d\n- Total replacements: %d\n\n## Modified Files\n",
		len(result.FilesChanged), result.Replacements)

	for _, filename := range result.FilesChanged {
		body += fmt.Sprintf("- %s\n", filename)
	}

	return body
}

func buildSearchSummary(opts github.SearchOptions) string {
	var parts []string

	if opts.Query != "" {
		parts = append(parts, fmt.Sprintf("query:'%s'", opts.Query))
	}
	if opts.Owner != "" {
		parts = append(parts, fmt.Sprintf("owner:%s", opts.Owner))
	}
	if opts.Language != "" {
		parts = append(parts, fmt.Sprintf("language:%s", opts.Language))
	}
	if opts.Stars != "" {
		parts = append(parts, fmt.Sprintf("stars:%s", opts.Stars))
	}

	if len(parts) == 0 {
		return "all repositories"
	}

	return strings.Join(parts, ", ")
}

func hasRepoAccess(repoFullName string, cfg *config.Config) bool {
	// Simple heuristic: if repo belongs to configured org, assume we have access
	if cfg.GitHub.Org != "" && strings.HasPrefix(repoFullName, cfg.GitHub.Org+"/") {
		return true
	}
	// For now, assume we have access to public repos
	// In a real implementation, you might want to test access by making a small API call
	return true
}
