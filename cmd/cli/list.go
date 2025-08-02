package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/utils"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List repositories accessible to the team",
	Long: `List all repositories that are accessible to the specified team.
This is useful for previewing which repositories will be affected by
the replace operation.`,
	RunE: runList,
}

var (
	showPrivate  bool
	outputFormat string
)

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().BoolVar(&showPrivate, "show-private", false, "Show private repositories")
	listCmd.Flags().StringVar(&outputFormat, "output", "table", "Output format (table, json)")
}

func runList(cmd *cobra.Command, args []string) error {
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

	if err := cfg.Validate(); err != nil {
		spinner.StopWithFailure("Configuration validation failed")
		return utils.NewValidationError("configuration validation failed", err)
	}

	spinner.StopWithSuccess("Configuration loaded")

	// Connect to GitHub with spinner
	spinner, err = utils.NewSpinner("Connecting to GitHub API")
	if err != nil {
		return err
	}

	if err := spinner.Start(); err != nil {
		return err
	}

	githubClient, err := github.NewClient(&github.Config{
		BaseURL:    cfg.GitHub.BaseURL,
		Token:      cfg.GitHub.Token,
		Timeout:    cfg.GitHub.Timeout,
		MaxRetries: cfg.GitHub.MaxRetries,
	})
	if err != nil {
		spinner.StopWithFailure("Failed to create GitHub client")
		return utils.NewAuthError("failed to create GitHub client", err)
	}

	spinner.UpdateMessage("Getting team information")

	team, err := githubClient.GetTeam(ctx, cfg.GitHub.Org, cfg.GitHub.Team)
	if err != nil {
		spinner.StopWithFailure("Failed to get team information")
		return utils.NewNetworkError("failed to get team information", err)
	}

	spinner.UpdateMessage("Fetching team repositories")

	repositories, err := githubClient.ListTeamRepositories(ctx, team.ID)
	if err != nil {
		spinner.StopWithFailure("Failed to list team repositories")
		return utils.NewNetworkError("failed to list team repositories", err)
	}

	spinner.StopWithSuccess(fmt.Sprintf("Retrieved %d repositories", len(repositories)))
	logger.Info("Retrieved repositories", "count", len(repositories))

	if outputFormat == "json" {
		return outputRepositoriesJSON(repositories, showPrivate)
	}

	outputRepositoriesTable(repositories, showPrivate)
	return nil
}

func outputRepositoriesTable(repositories []*github.Repository, showPrivate bool) {
	fmt.Printf("Repositories accessible to the team:\n\n")
	fmt.Printf("%-40s %-10s %-50s\n", "NAME", "VISIBILITY", "CLONE URL")
	fmt.Printf("%-40s %-10s %-50s\n", "----", "----------", "---------")

	visibleCount := 0
	for _, repo := range repositories {
		if repo.Private && !showPrivate {
			continue
		}

		visibility := "public"
		if repo.Private {
			visibility = "private"
		}

		fmt.Printf("%-40s %-10s %-50s\n", repo.FullName, visibility, repo.CloneURL)
		visibleCount++
	}

	fmt.Printf("\nTotal: %d repositories", visibleCount)
	if !showPrivate {
		privateCount := len(repositories) - visibleCount
		if privateCount > 0 {
			fmt.Printf(" (%d private repositories hidden, use --show-private to see all)", privateCount)
		}
	}
	fmt.Println()
}

func outputRepositoriesJSON(repositories []*github.Repository, showPrivate bool) error {
	fmt.Printf("[\n")

	first := true
	for _, repo := range repositories {
		if repo.Private && !showPrivate {
			continue
		}

		if !first {
			fmt.Printf(",\n")
		}
		first = false

		visibility := "public"
		if repo.Private {
			visibility = "private"
		}

		fmt.Printf("  {\n")
		fmt.Printf("    \"id\": %d,\n", repo.ID)
		fmt.Printf("    \"name\": \"%s\",\n", repo.Name)
		fmt.Printf("    \"full_name\": \"%s\",\n", repo.FullName)
		fmt.Printf("    \"clone_url\": \"%s\",\n", repo.CloneURL)
		fmt.Printf("    \"ssh_url\": \"%s\",\n", repo.SSHURL)
		fmt.Printf("    \"visibility\": \"%s\"\n", visibility)
		fmt.Printf("  }")
	}

	fmt.Printf("\n]\n")
	return nil
}
