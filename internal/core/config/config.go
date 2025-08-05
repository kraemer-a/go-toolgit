package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	GitHub      GitHubConfig      `yaml:"github" mapstructure:"github"`
	Bitbucket   BitbucketConfig   `yaml:"bitbucket" mapstructure:"bitbucket"`
	Provider    string            `yaml:"provider" mapstructure:"provider"` // "github" or "bitbucket"
	Processing  ProcessingConfig  `yaml:"processing" mapstructure:"processing"`
	PullRequest PullRequestConfig `yaml:"pull_request" mapstructure:"pull_request"`
	Logging     LoggingConfig     `yaml:"logging" mapstructure:"logging"`
}

type GitHubConfig struct {
	BaseURL          string        `yaml:"base_url" mapstructure:"base_url" validate:"required,url"`
	Token            string        `yaml:"token" mapstructure:"token" validate:"required"`
	Org              string        `yaml:"organization" mapstructure:"organization" validate:"required"`
	Team             string        `yaml:"team" mapstructure:"team" validate:"required"`
	Timeout          time.Duration `yaml:"timeout" mapstructure:"timeout"`
	MaxRetries       int           `yaml:"max_retries" mapstructure:"max_retries"`
	WaitForRateLimit bool          `yaml:"wait_for_rate_limit" mapstructure:"wait_for_rate_limit"`
}

type BitbucketConfig struct {
	BaseURL    string        `yaml:"base_url" mapstructure:"base_url" validate:"required,url"`
	Username   string        `yaml:"username" mapstructure:"username" validate:"required"`
	Password   string        `yaml:"password" mapstructure:"password" validate:"required"` // Personal Access Token or password
	Project    string        `yaml:"project" mapstructure:"project" validate:"required"`   // Bitbucket project key
	Timeout    time.Duration `yaml:"timeout" mapstructure:"timeout"`
	MaxRetries int           `yaml:"max_retries" mapstructure:"max_retries"`
}

type ProcessingConfig struct {
	IncludePatterns []string `yaml:"include_patterns" mapstructure:"include_patterns"`
	ExcludePatterns []string `yaml:"exclude_patterns" mapstructure:"exclude_patterns"`
	MaxWorkers      int      `yaml:"max_workers" mapstructure:"max_workers"`
}

type PullRequestConfig struct {
	TitleTemplate string `yaml:"title_template" mapstructure:"title_template"`
	BodyTemplate  string `yaml:"body_template" mapstructure:"body_template"`
	BranchPrefix  string `yaml:"branch_prefix" mapstructure:"branch_prefix"`
	AutoMerge     bool   `yaml:"auto_merge" mapstructure:"auto_merge"`
	DeleteBranch  bool   `yaml:"delete_branch" mapstructure:"delete_branch"`
}

type LoggingConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`
	Format string `yaml:"format" mapstructure:"format"`
}

type ReplacementRule struct {
	Original      string `yaml:"original" validate:"required"`
	Replacement   string `yaml:"replacement"`
	Regex         bool   `yaml:"regex"`
	CaseSensitive bool   `yaml:"case_sensitive"`
	WholeWord     bool   `yaml:"whole_word"`
}

func Load() (*Config, error) {
	// Use the global viper instance that was already configured by the root command
	// Don't reconfigure viper here since it was already done in root.go

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// LoadSecure loads configuration with automatic decryption of sensitive fields
func LoadSecure() (*Config, error) {
	scm, err := NewSecureConfigManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create secure config manager: %w", err)
	}

	return scm.LoadSecureConfig()
}

func setDefaults() {
	viper.SetDefault("github.base_url", "https://api.github.com")
	viper.SetDefault("github.timeout", "30s")
	viper.SetDefault("github.max_retries", 3)
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

func (c *Config) Validate() error {
	// Default to GitHub if no provider is specified
	if c.Provider == "" {
		c.Provider = "github"
	}

	switch c.Provider {
	case "github":
		return c.validateGitHub()
	case "bitbucket":
		return c.validateBitbucket()
	default:
		return fmt.Errorf("unsupported provider: %s (supported: github, bitbucket)", c.Provider)
	}
}

func (c *Config) validateGitHub() error {
	if c.GitHub.BaseURL == "" {
		return fmt.Errorf("github.base_url is required")
	}
	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	if c.GitHub.Org == "" {
		return fmt.Errorf("github.organization is required")
	}
	if c.GitHub.Team == "" {
		return fmt.Errorf("github.team is required")
	}
	return nil
}

func (c *Config) validateBitbucket() error {
	if c.Bitbucket.BaseURL == "" {
		return fmt.Errorf("bitbucket.base_url is required")
	}
	if c.Bitbucket.Username == "" {
		return fmt.Errorf("bitbucket.username is required")
	}
	if c.Bitbucket.Password == "" {
		return fmt.Errorf("bitbucket.password is required")
	}
	if c.Bitbucket.Project == "" {
		return fmt.Errorf("bitbucket.project is required")
	}
	return nil
}

// ValidateForSearch validates configuration for search operations where org/team are optional
func (c *Config) ValidateForSearch() error {
	// Default to GitHub if no provider is specified
	if c.Provider == "" {
		c.Provider = "github"
	}

	switch c.Provider {
	case "github":
		return c.validateGitHubForSearch()
	case "bitbucket":
		return c.validateBitbucketForSearch()
	default:
		return fmt.Errorf("unsupported provider: %s (supported: github, bitbucket)", c.Provider)
	}
}

func (c *Config) validateGitHubForSearch() error {
	if c.GitHub.BaseURL == "" {
		return fmt.Errorf("github.base_url is required")
	}
	if c.GitHub.Token == "" {
		return fmt.Errorf("github.token is required")
	}
	// org and team are optional for search operations
	return nil
}

func (c *Config) validateBitbucketForSearch() error {
	if c.Bitbucket.BaseURL == "" {
		return fmt.Errorf("bitbucket.base_url is required")
	}
	if c.Bitbucket.Username == "" {
		return fmt.Errorf("bitbucket.username is required")
	}
	if c.Bitbucket.Password == "" {
		return fmt.Errorf("bitbucket.password is required")
	}
	// project is optional for search operations
	return nil
}
