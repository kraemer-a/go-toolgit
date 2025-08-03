package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/utils"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and GitHub access",
	Long: `Validate that the configuration is correct and that the tool can access
the specified GitHub organization and team with the provided credentials.

This command will:
1. Load and validate the configuration
2. Test GitHub API access
3. Verify team membership and permissions`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	logger := GetLogger()
	ctx := context.Background()

	// Create and start spinner for configuration loading
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

	spinner.StopWithSuccess("Configuration loaded and validated")
	logger.Info("Configuration validation passed")

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

	spinner.UpdateMessage("Validating GitHub access")

	if err := githubClient.ValidateAccess(ctx, cfg.GitHub.Org, cfg.GitHub.Team); err != nil {
		spinner.StopWithFailure("GitHub access validation failed")
		return utils.NewAuthError("failed to validate GitHub access", err)
	}

	spinner.StopWithSuccess("GitHub API connection established")

	// Get team information with spinner
	spinner, err = utils.NewSpinner("Retrieving team information")
	if err != nil {
		return err
	}

	if err := spinner.Start(); err != nil {
		return err
	}

	team, err := githubClient.GetTeam(ctx, cfg.GitHub.Org, cfg.GitHub.Team)
	if err != nil {
		spinner.StopWithFailure("Failed to get team information")
		return utils.NewNetworkError("failed to get team information", err)
	}

	spinner.StopWithSuccess("Team information retrieved")
	logger.Info("Team access validated",
		"team_id", team.ID,
		"team_name", team.Name,
		"team_slug", team.Slug)

	// List repositories with spinner
	spinner, err = utils.NewSpinner("Listing team repositories")
	if err != nil {
		return err
	}

	if err := spinner.Start(); err != nil {
		return err
	}

	repositories, err := githubClient.ListTeamRepositories(ctx, team.ID)
	if err != nil {
		spinner.StopWithFailure("Failed to list team repositories")
		return utils.NewNetworkError("failed to list team repositories", err)
	}

	spinner.StopWithSuccess(fmt.Sprintf("Found %d accessible repositories", len(repositories)))
	logger.Info("Repository access validated",
		"repository_count", len(repositories))

	fmt.Printf("\n✓ Validation successful!\n")
	fmt.Printf("  - Organization: %s\n", cfg.GitHub.Org)
	fmt.Printf("  - Team: %s (%s)\n", team.Name, team.Slug)
	fmt.Printf("  - Accessible repositories: %d\n", len(repositories))
	fmt.Printf("  - GitHub URL: %s\n", cfg.GitHub.BaseURL)

	return nil
}
