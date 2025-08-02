package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "ghp_test_token",
					Org:     "test-org",
					Team:    "test-team",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing base URL",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "",
					Token:   "ghp_test_token",
					Org:     "test-org",
					Team:    "test-team",
				},
			},
			wantErr: true,
			errMsg:  "github.base_url is required",
		},
		{
			name: "Missing token",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "",
					Org:     "test-org",
					Team:    "test-team",
				},
			},
			wantErr: true,
			errMsg:  "github.token is required",
		},
		{
			name: "Missing organization",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "ghp_test_token",
					Org:     "",
					Team:    "test-team",
				},
			},
			wantErr: true,
			errMsg:  "github.organization is required",
		},
		{
			name: "Missing team",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "ghp_test_token",
					Org:     "test-org",
					Team:    "",
				},
			},
			wantErr: true,
			errMsg:  "github.team is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("Config.Validate() error = %v, expected %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestConfig_ValidateForSearch(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config for search",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "ghp_test_token",
					// Org and Team are optional for search
				},
			},
			wantErr: false,
		},
		{
			name: "Missing base URL",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "",
					Token:   "ghp_test_token",
				},
			},
			wantErr: true,
			errMsg:  "github.base_url is required",
		},
		{
			name: "Missing token",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "",
				},
			},
			wantErr: true,
			errMsg:  "github.token is required",
		},
		{
			name: "Valid with empty org and team",
			config: &Config{
				GitHub: GitHubConfig{
					BaseURL: "https://api.github.com",
					Token:   "ghp_test_token",
					Org:     "",
					Team:    "",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateForSearch()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.ValidateForSearch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("Config.ValidateForSearch() error = %v, expected %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestConfigStructs_Completeness(t *testing.T) {
	config := &Config{
		GitHub: GitHubConfig{
			BaseURL:    "https://api.github.com",
			Token:      "ghp_test_token",
			Org:        "test-org",
			Team:       "test-team",
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		},
		Processing: ProcessingConfig{
			IncludePatterns: []string{"*.go", "*.js"},
			ExcludePatterns: []string{"vendor/*", "*_test.go"},
			MaxWorkers:      4,
		},
		PullRequest: PullRequestConfig{
			TitleTemplate: "feat: automated replacement",
			BodyTemplate:  "Automated changes by go-toolgit tool",
			BranchPrefix:  "auto-replace",
			AutoMerge:     false,
			DeleteBranch:  true,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}

	// Test that all fields are accessible
	if config.GitHub.BaseURL != "https://api.github.com" {
		t.Errorf("GitHub BaseURL not set correctly")
	}

	if len(config.Processing.IncludePatterns) != 2 {
		t.Errorf("Processing IncludePatterns should have 2 items, got %d", len(config.Processing.IncludePatterns))
	}

	if config.PullRequest.AutoMerge {
		t.Errorf("PullRequest AutoMerge should be false")
	}

	if config.Logging.Level != "info" {
		t.Errorf("Logging Level should be 'info', got %q", config.Logging.Level)
	}
}

func TestReplacementRule_Struct(t *testing.T) {
	rule := ReplacementRule{
		Original:      "old_value",
		Replacement:   "new_value",
		Regex:         true,
		CaseSensitive: false,
		WholeWord:     true,
	}

	if rule.Original != "old_value" {
		t.Errorf("Expected Original 'old_value', got %q", rule.Original)
	}

	if rule.Replacement != "new_value" {
		t.Errorf("Expected Replacement 'new_value', got %q", rule.Replacement)
	}

	if !rule.Regex {
		t.Errorf("Expected Regex to be true")
	}

	if rule.CaseSensitive {
		t.Errorf("Expected CaseSensitive to be false")
	}

	if !rule.WholeWord {
		t.Errorf("Expected WholeWord to be true")
	}
}

func TestConfigDefaults(t *testing.T) {
	// Clear any existing viper settings
	viper.Reset()

	// Set defaults as the function would
	viper.SetDefault("github.base_url", "https://api.github.com")
	viper.SetDefault("github.timeout", "30s")
	viper.SetDefault("github.max_retries", 3)

	// Test that defaults are set correctly
	if viper.GetString("github.base_url") != "https://api.github.com" {
		t.Errorf("Default base_url not set correctly")
	}

	if viper.GetString("github.timeout") != "30s" {
		t.Errorf("Default timeout not set correctly")
	}

	if viper.GetInt("github.max_retries") != 3 {
		t.Errorf("Default max_retries not set correctly")
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Test environment variable handling
	originalToken := os.Getenv("GITHUB_REPLACE_GITHUB_TOKEN")
	defer func() {
		if originalToken != "" {
			os.Setenv("GITHUB_REPLACE_GITHUB_TOKEN", originalToken)
		} else {
			os.Unsetenv("GITHUB_REPLACE_GITHUB_TOKEN")
		}
	}()

	// Set test environment variable
	testToken := "test_token_12345"
	os.Setenv("GITHUB_REPLACE_GITHUB_TOKEN", testToken)

	// Reset viper and set up environment
	viper.Reset()
	viper.SetEnvPrefix("GITHUB_REPLACE")
	viper.AutomaticEnv()

	// Viper converts env var GITHUB_REPLACE_GITHUB_TOKEN to key "github_token"
	// when using AutomaticEnv with prefix
	actualToken := viper.GetString("github_token") // env var is GITHUB_REPLACE_GITHUB_TOKEN
	if actualToken != testToken {
		t.Errorf("Environment variable not read correctly, expected %q, got %q",
			testToken, actualToken)
	}
}

// Test mapstructure tags are working correctly
func TestMapstructureTags(t *testing.T) {
	viper.Reset()

	// Set values in viper
	viper.Set("github.base_url", "https://test.github.com")
	viper.Set("github.token", "test_token")
	viper.Set("github.organization", "test_org")
	viper.Set("github.team", "test_team")
	viper.Set("github.timeout", "45s")
	viper.Set("github.max_retries", 5)

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	// Verify that mapstructure tags work correctly
	if config.GitHub.BaseURL != "https://test.github.com" {
		t.Errorf("BaseURL not unmarshaled correctly, got %q", config.GitHub.BaseURL)
	}

	if config.GitHub.Token != "test_token" {
		t.Errorf("Token not unmarshaled correctly, got %q", config.GitHub.Token)
	}

	if config.GitHub.Org != "test_org" {
		t.Errorf("Org not unmarshaled correctly, got %q", config.GitHub.Org)
	}

	if config.GitHub.Team != "test_team" {
		t.Errorf("Team not unmarshaled correctly, got %q", config.GitHub.Team)
	}

	if config.GitHub.Timeout != 45*time.Second {
		t.Errorf("Timeout not unmarshaled correctly, got %v", config.GitHub.Timeout)
	}

	if config.GitHub.MaxRetries != 5 {
		t.Errorf("MaxRetries not unmarshaled correctly, got %d", config.GitHub.MaxRetries)
	}
}
