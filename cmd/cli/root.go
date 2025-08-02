package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go-toolgit/internal/core/config"
	"go-toolgit/internal/core/utils"
)

var (
	cfgFile string
	logger  *utils.Logger
)

var rootCmd = &cobra.Command{
	Use:   "go-toolgit",
	Short: "DevOps tool for string replacements and repository migrations",
	Long: `A comprehensive CLI and GUI tool for automated string replacements and repository 
migrations across multiple repositories on GitHub and Bitbucket Server.

Features:
• String replacement across multiple repositories
• Repository migration from Bitbucket Server to GitHub
• Dual GUI support: Wails web interface and Fyne native interface
• Team management and webhook configuration

GUI Options:
  --gui       Launch Wails web-based GUI (requires: wails build)
  --fyne-gui  Launch Fyne native GUI (works with standard build)

The tool can connect to on-premise GitHub instances and create pull requests
with the changes automatically.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.go-toolgit/config.yaml)")
	rootCmd.PersistentFlags().String("provider", "github", "Git provider (github, bitbucket)")

	// GitHub flags
	rootCmd.PersistentFlags().String("github-url", "", "GitHub base URL for on-premise instances")
	rootCmd.PersistentFlags().String("token", "", "GitHub personal access token")
	rootCmd.PersistentFlags().String("org", "", "GitHub organization name")
	rootCmd.PersistentFlags().String("team", "", "GitHub team name")

	// Bitbucket flags
	rootCmd.PersistentFlags().String("bitbucket-url", "", "Bitbucket Server base URL")
	rootCmd.PersistentFlags().String("bitbucket-username", "", "Bitbucket username")
	rootCmd.PersistentFlags().String("bitbucket-password", "", "Bitbucket password or personal access token")
	rootCmd.PersistentFlags().String("bitbucket-project", "", "Bitbucket project key")

	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "text", "Log format (text, json)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode with verbose logging")
	rootCmd.PersistentFlags().Bool("gui", false, "Launch GUI interface")

	viper.BindPFlag("provider", rootCmd.PersistentFlags().Lookup("provider"))

	// GitHub bindings
	viper.BindPFlag("github.base_url", rootCmd.PersistentFlags().Lookup("github-url"))
	viper.BindPFlag("github.token", rootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("github.organization", rootCmd.PersistentFlags().Lookup("org"))
	viper.BindPFlag("github.team", rootCmd.PersistentFlags().Lookup("team"))

	// Bitbucket bindings
	viper.BindPFlag("bitbucket.base_url", rootCmd.PersistentFlags().Lookup("bitbucket-url"))
	viper.BindPFlag("bitbucket.username", rootCmd.PersistentFlags().Lookup("bitbucket-username"))
	viper.BindPFlag("bitbucket.password", rootCmd.PersistentFlags().Lookup("bitbucket-password"))
	viper.BindPFlag("bitbucket.project", rootCmd.PersistentFlags().Lookup("bitbucket-project"))

	viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("logging.format", rootCmd.PersistentFlags().Lookup("log-format"))
}

func initConfig() {
	// Set defaults first
	setDefaults()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		viper.AddConfigPath(home + "/.go-toolgit")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("GITHUB_REPLACE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// Only show config file info in debug mode
		if debugFlag, _ := rootCmd.PersistentFlags().GetBool("debug"); debugFlag {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

func setDefaults() {
	viper.SetDefault("provider", "github")

	// GitHub defaults
	viper.SetDefault("github.base_url", "https://api.github.com")
	viper.SetDefault("github.timeout", "30s")
	viper.SetDefault("github.max_retries", 3)

	// Bitbucket defaults
	viper.SetDefault("bitbucket.timeout", "30s")
	viper.SetDefault("bitbucket.max_retries", 3)

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

func initializeConfig() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger = utils.NewLogger(cfg.Logging.Level, cfg.Logging.Format)
	return nil
}

func GetLogger() *utils.Logger {
	return logger
}

func IsDebugMode() bool {
	debugFlag, _ := rootCmd.PersistentFlags().GetBool("debug")
	logLevel, _ := rootCmd.PersistentFlags().GetString("log-level")
	return debugFlag || logLevel == "debug"
}
