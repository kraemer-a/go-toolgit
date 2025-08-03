package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/utils"
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for repositories on GitHub",
	Long: `Search for repositories on GitHub using various criteria.
This allows you to find repositories beyond just organization/team membership.

Examples:
  # Search by owner/organization
  go-toolgit search --owner gurke

  # Search by language and stars
  go-toolgit search --language go --stars ">100"

  # Complex search query
  go-toolgit search --query "microservice" --language go --owner myorg --stars "10..100"

  # Search with sorting
  go-toolgit search --owner gurke --sort stars --order desc --limit 50`,
	RunE: runSearch,
}

var (
	searchQuery     string
	searchOwner     string
	searchLanguage  string
	searchStars     string
	searchSize      string
	includeForks    bool
	includeArchived bool
	sortBy          string
	sortOrder       string
	searchLimit     int
)

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().StringVar(&searchQuery, "query", "", "Search query (keywords)")
	searchCmd.Flags().StringVar(&searchOwner, "owner", "", "Repository owner (user or organization)")
	searchCmd.Flags().StringVar(&searchLanguage, "language", "", "Programming language")
	searchCmd.Flags().StringVar(&searchStars, "stars", "", "Star count (e.g., '>100', '50..100')")
	searchCmd.Flags().StringVar(&searchSize, "size", "", "Repository size in KB (e.g., '>1000')")
	searchCmd.Flags().BoolVar(&includeForks, "include-forks", false, "Include forked repositories")
	searchCmd.Flags().BoolVar(&includeArchived, "include-archived", false, "Include archived repositories")
	searchCmd.Flags().StringVar(&sortBy, "sort", "updated", "Sort by: stars, forks, updated")
	searchCmd.Flags().StringVar(&sortOrder, "order", "desc", "Sort order: asc, desc")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 100, "Maximum number of results")
	searchCmd.Flags().StringVar(&outputFormat, "output", "table", "Output format (table, json)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	ctx := context.Background()

	// Load configuration with spinner
	spinner, err := utils.NewSpinner("Loading configuration")
	if err != nil {
		return err
	}

	if err := spinner.Start(); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		spinner.StopWithFailure("Failed to load configuration")
		return utils.NewValidationError("failed to load configuration", err)
	}

	if err := cfg.ValidateForSearch(); err != nil {
		spinner.StopWithFailure("Configuration validation failed")
		return utils.NewValidationError("configuration validation failed", err)
	}

	spinner.StopWithSuccess("Configuration loaded")

	// Create GitHub client with spinner
	spinner, err = utils.NewSpinner("Connecting to GitHub API")
	if err != nil {
		return err
	}

	if err := spinner.Start(); err != nil {
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
		spinner.StopWithFailure("Failed to create GitHub client")
		return utils.NewAuthError("failed to create GitHub client", err)
	}

	spinner.StopWithSuccess("Connected to GitHub API")

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
		MaxResults: searchLimit,
	}

	// Validate search criteria
	if searchQuery == "" && searchOwner == "" && searchLanguage == "" && searchStars == "" && searchSize == "" {
		return fmt.Errorf("at least one search criterion must be specified (query, owner, language, stars, or size)")
	}

	// Search repositories with spinner
	spinner, err = utils.NewSpinner("Searching repositories")
	if err != nil {
		return err
	}

	if err := spinner.Start(); err != nil {
		return err
	}

	repositories, err := githubClient.SearchRepositories(ctx, searchOpts)
	if err != nil {
		spinner.StopWithFailure("Repository search failed")
		return utils.NewNetworkError("failed to search repositories", err)
	}

	spinner.StopWithSuccess(fmt.Sprintf("Found %d repositories", len(repositories)))
	logger.Info("Retrieved repositories", "count", len(repositories))

	// Display results
	if outputFormat == "json" {
		return outputRepositoriesJSON(repositories, true) // Show all including private
	}

	outputSearchResults(repositories, searchOpts)
	return nil
}

func outputSearchResults(repositories []*github.Repository, opts github.SearchOptions) {
	fmt.Printf("Search Results\n")
	fmt.Printf("==============\n\n")

	// Show search criteria
	fmt.Printf("Search Criteria:\n")
	if opts.Query != "" {
		fmt.Printf("  Query: %s\n", opts.Query)
	}
	if opts.Owner != "" {
		fmt.Printf("  Owner: %s\n", opts.Owner)
	}
	if opts.Language != "" {
		fmt.Printf("  Language: %s\n", opts.Language)
	}
	if opts.Stars != "" {
		fmt.Printf("  Stars: %s\n", opts.Stars)
	}
	if opts.Size != "" {
		fmt.Printf("  Size: %s KB\n", opts.Size)
	}
	if opts.Fork {
		fmt.Printf("  Including forks: yes\n")
	}
	if opts.Archived {
		fmt.Printf("  Including archived: yes\n")
	}
	fmt.Printf("  Sort: %s (%s)\n", opts.Sort, opts.Order)
	fmt.Printf("  Limit: %d\n\n", opts.MaxResults)

	if len(repositories) == 0 {
		fmt.Printf("No repositories found matching the search criteria.\n")
		return
	}

	// Display results table
	fmt.Printf("%-40s %-10s %-50s\n", "REPOSITORY", "VISIBILITY", "CLONE URL")
	fmt.Printf("%-40s %-10s %-50s\n", strings.Repeat("-", 40), strings.Repeat("-", 10), strings.Repeat("-", 50))

	for _, repo := range repositories {
		visibility := "public"
		if repo.Private {
			visibility = "private"
		}

		fmt.Printf("%-40s %-10s %-50s\n", repo.FullName, visibility, repo.CloneURL)
	}

	fmt.Printf("\nTotal: %d repositories found\n", len(repositories))

	// Show usage tip
	fmt.Printf("\nTip: Use these repositories with the replace command:\n")
	fmt.Printf("  go-toolgit replace-search --search-query \"your-query\" --replacements \"old=new\"\n")
}

// Helper function to validate search parameters
func validateSearchStars(stars string) error {
	if stars == "" {
		return nil
	}

	// Check for range format (e.g., "10..100")
	if strings.Contains(stars, "..") {
		parts := strings.Split(stars, "..")
		if len(parts) != 2 {
			return fmt.Errorf("invalid stars range format: %s (use 'min..max')", stars)
		}

		for _, part := range parts {
			if _, err := strconv.Atoi(part); err != nil {
				return fmt.Errorf("invalid stars range value: %s", part)
			}
		}
		return nil
	}

	// Check for comparison format (e.g., ">100", "<=50")
	if len(stars) > 1 && (stars[0] == '>' || stars[0] == '<' || stars[0] == '=') {
		numStr := stars[1:]
		if stars[0] == '<' && len(stars) > 2 && stars[1] == '=' {
			numStr = stars[2:]
		} else if stars[0] == '>' && len(stars) > 2 && stars[1] == '=' {
			numStr = stars[2:]
		}

		if _, err := strconv.Atoi(numStr); err != nil {
			return fmt.Errorf("invalid stars comparison value: %s", numStr)
		}
		return nil
	}

	// Check for exact number
	if _, err := strconv.Atoi(stars); err != nil {
		return fmt.Errorf("invalid stars value: %s", stars)
	}

	return nil
}
