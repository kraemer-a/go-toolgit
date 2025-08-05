package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/github"
	"go-toolgit/internal/core/utils"
)

var rateLimitCmd = &cobra.Command{
	Use:   "rate-limit",
	Short: "Display GitHub API rate limit information",
	Long: `Display current GitHub API rate limit status including:
- Core API rate limits (5000 requests/hour)
- Search API rate limits (30 requests/minute)
- Time until rate limit reset`,
	RunE: runRateLimit,
}

func init() {
	rootCmd.AddCommand(rateLimitCmd)
}

func runRateLimit(cmd *cobra.Command, args []string) error {
	logger := GetLogger()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return utils.NewValidationError("failed to load configuration", err)
	}

	// Create GitHub client
	githubClient, err := github.NewClient(&github.Config{
		BaseURL:      cfg.GitHub.BaseURL,
		Token:        cfg.GitHub.Token,
		Timeout:      cfg.GitHub.Timeout,
		MaxRetries:   cfg.GitHub.MaxRetries,
		WaitForReset: cfg.GitHub.WaitForRateLimit,
	})
	if err != nil {
		return utils.NewAuthError("failed to create GitHub client", err)
	}

	// Get rate limit info
	info := githubClient.GetRateLimitInfo()

	// Display rate limit information
	fmt.Println("\n📊 GitHub API Rate Limits:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Core API
	fmt.Printf("\n🔷 Core API:\n")
	fmt.Printf("   Limit:     %d requests/hour\n", info.Core.Limit)
	fmt.Printf("   Remaining: %d requests\n", info.Core.Remaining)
	fmt.Printf("   Reset:     %s (%s)\n", 
		info.Core.Reset.Format(time.RFC3339),
		formatTimeUntil(info.Core.Reset))
	
	// Search API
	fmt.Printf("\n🔍 Search API:\n")
	fmt.Printf("   Limit:     %d requests/minute\n", info.Search.Limit)
	fmt.Printf("   Remaining: %d requests\n", info.Search.Remaining)
	fmt.Printf("   Reset:     %s (%s)\n", 
		info.Search.Reset.Format(time.RFC3339),
		formatTimeUntil(info.Search.Reset))
	
	// Configuration
	fmt.Printf("\n⚙️  Configuration:\n")
	fmt.Printf("   Wait for rate limit: %v\n", cfg.GitHub.WaitForRateLimit)
	
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Log the information
	logger.Info("Rate limit check completed",
		"core_remaining", info.Core.Remaining,
		"search_remaining", info.Search.Remaining)
	
	// Warnings
	if info.Core.Remaining < 100 {
		fmt.Printf("\n⚠️  Warning: Core API rate limit is low (%d remaining)\n", info.Core.Remaining)
	}
	if info.Search.Remaining < 5 {
		fmt.Printf("\n⚠️  Warning: Search API rate limit is low (%d remaining)\n", info.Search.Remaining)
	}
	
	return nil
}

func formatTimeUntil(t time.Time) string {
	duration := time.Until(t)
	if duration < 0 {
		return "expired"
	}
	
	if duration < time.Minute {
		return fmt.Sprintf("in %d seconds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("in %d minutes", int(duration.Minutes()))
	}
	return fmt.Sprintf("in %.1f hours", duration.Hours())
}