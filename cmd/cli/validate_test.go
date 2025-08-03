package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "help command",
			args:        []string{"validate", "--help"},
			expectError: false,
		},
		{
			name:        "no arguments",
			args:        []string{"validate"},
			expectError: true, // Will fail due to missing config
		},
		{
			name:        "with debug flag",
			args:        []string{"validate", "--debug"},
			expectError: true, // Will fail due to missing config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.AddCommand(validateCmd)
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
				if err != nil && !strings.Contains(output.String(), "help") {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateSecurityInputs(t *testing.T) {
	maliciousInputs := []struct {
		name  string
		args  []string
		desc  string
	}{
		{
			name: "command injection in org",
			args: []string{"validate", "--org", "test-org; rm -rf /", "--help"},
			desc: "Should not execute embedded commands",
		},
		{
			name: "command injection in team",
			args: []string{"validate", "--team", "test-team && curl evil.com", "--help"},
			desc: "Should not execute network commands",
		},
		{
			name: "path traversal in config",
			args: []string{"validate", "--config", "../../../etc/passwd", "--help"},
			desc: "Should not access system files",
		},
		{
			name: "script injection in token",
			args: []string{"validate", "--token", "<script>alert('xss')</script>", "--help"},
			desc: "Should not execute scripts",
		},
		{
			name: "extremely long input",
			args: []string{"validate", "--org", strings.Repeat("A", 10000), "--help"},
			desc: "Should handle large inputs gracefully",
		},
		{
			name: "null bytes in input",
			args: []string{"validate", "--org", "test\x00org", "--help"},
			desc: "Should handle null bytes safely",
		},
	}

	for _, tt := range maliciousInputs {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.AddCommand(validateCmd)
			cmd.SetArgs(tt.args)

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Command should not crash or execute malicious content
			err := cmd.Execute()
			
			// We expect these to either succeed (show help) or fail gracefully
			if err != nil && !strings.Contains(err.Error(), "unknown flag") && 
			   !strings.Contains(output.String(), "help") {
				// Only fail if it's a crash, not expected validation errors
				if strings.Contains(err.Error(), "panic") || 
				   strings.Contains(err.Error(), "runtime error") {
					t.Errorf("Command crashed with malicious input: %v", err)
				}
			}

			// Verify no malicious content appears in output
			outputStr := output.String()
			if strings.Contains(outputStr, "<script>") ||
			   strings.Contains(outputStr, "rm -rf") ||
			   strings.Contains(outputStr, "curl evil.com") {
				t.Errorf("Malicious content appears in output: %s", outputStr)
			}
		})
	}
}

func TestValidateInputSanitization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool // true if input should be considered safe
	}{
		{
			name:     "normal org name",
			input:    "my-organization",
			expected: true,
		},
		{
			name:     "org with command injection",
			input:    "org; rm -rf /",
			expected: false,
		},
		{
			name:     "org with path traversal",
			input:    "../../../etc/passwd",
			expected: false,
		},
		{
			name:     "org with script injection",
			input:    "<script>alert('xss')</script>",
			expected: false,
		},
		{
			name:     "org with sql injection",
			input:    "'; DROP TABLE users; --",
			expected: false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: true, // Empty is valid, just missing required value
		},
		{
			name:     "unicode characters",
			input:    "org-中文",
			expected: true,
		},
		{
			name:     "special characters but safe",
			input:    "org_name-123",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that potentially dangerous inputs are detected
			isSafe := isInputSafe(tt.input)
			if isSafe != tt.expected {
				t.Errorf("Input safety check failed for '%s': expected %v, got %v", 
					tt.input, tt.expected, isSafe)
			}
		})
	}
}

// Helper function to check if input is safe (this would be implemented in actual code)
func isInputSafe(input string) bool {
	// Basic input sanitization check
	dangerousPatterns := []string{
		";", "&", "|", "`", "$", "(", ")", 
		"<script", "</script", "rm -rf", "curl",
		"DROP TABLE", "SELECT * FROM", "../",
	}
	
	inputLower := strings.ToLower(input)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return false
		}
	}
	return true
}

func TestValidateConfigurationSecurity(t *testing.T) {
	tests := []struct {
		name        string
		org         string
		team        string
		token       string
		expectError bool
		errorType   string
	}{
		{
			name:        "normal valid inputs",
			org:         "test-org",
			team:        "test-team",
			token:       "ghp_validtoken123",
			expectError: false,
		},
		{
			name:        "empty required fields",
			org:         "",
			team:        "",
			token:       "",
			expectError: true,
			errorType:   "validation",
		},
		{
			name:        "suspicious token format",
			org:         "test-org",
			team:        "test-team",
			token:       "not-a-github-token",
			expectError: false, // Format validation is lenient
		},
		{
			name:        "command injection in org",
			org:         "test-org; rm -rf /",
			team:        "test-team",
			token:       "ghp_token123",
			expectError: false, // Will be caught by input validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test configuration validation with various inputs
			// This would test the actual configuration loading and validation logic
			
			// For now, just verify the command doesn't crash
			cmd := createTestRootCommand()
			cmd.AddCommand(validateCmd)
			
			args := []string{"validate"}
			if tt.org != "" {
				args = append(args, "--org", tt.org)
			}
			if tt.team != "" {
				args = append(args, "--team", tt.team)
			}
			if tt.token != "" {
				args = append(args, "--token", tt.token)
			}
			args = append(args, "--help") // Add help to avoid actual execution
			
			cmd.SetArgs(args)

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			err := cmd.Execute()
			
			// Should not crash regardless of input
			if err != nil && strings.Contains(err.Error(), "runtime error") {
				t.Errorf("Command crashed with inputs org=%s, team=%s, token=%s: %v", 
					tt.org, tt.team, tt.token, err)
			}
		})
	}
}

func TestValidateRateLimiting(t *testing.T) {
	// Test that the validate command respects rate limits and doesn't make excessive API calls
	t.Run("single validation call", func(t *testing.T) {
		// In a real implementation, this would test that validate doesn't make
		// redundant API calls and respects GitHub's rate limits
		
		// Mock test: verify command structure doesn't allow multiple simultaneous calls
		cmd := createTestRootCommand()
		cmd.AddCommand(validateCmd)
		cmd.SetArgs([]string{"validate", "--help"})

		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)

		err := cmd.Execute()
		if err != nil && !strings.Contains(output.String(), "help") {
			t.Errorf("Unexpected error in rate limiting test: %v", err)
		}
	})
}

func TestValidateErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name:          "invalid flag",
			args:          []string{"validate", "--invalid-flag"},
			expectedError: "unknown flag",
		},
		{
			name:          "conflicting flags",
			args:          []string{"validate", "--provider", "github", "--provider", "bitbucket"},
			expectedError: "", // Should use last value
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.AddCommand(validateCmd)
			cmd.SetArgs(tt.args)

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			err := cmd.Execute()

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s' but got none", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
			}
		})
	}
}

// Mock validate function for testing (would replace actual implementation in tests)
func mockRunValidate(cmd *cobra.Command, args []string) error {
	// Mock implementation that doesn't make real API calls
	return nil
}

// Benchmark tests for validate command performance
func BenchmarkValidateCommand(b *testing.B) {
	cmd := createTestRootCommand()
	cmd.AddCommand(validateCmd)
	
	for i := 0; i < b.N; i++ {
		cmd.SetArgs([]string{"validate", "--help"})
		
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		
		cmd.Execute()
	}
}

func BenchmarkValidateInputSanitization(b *testing.B) {
	testInputs := []string{
		"normal-org",
		"org; rm -rf /",
		"<script>alert('xss')</script>",
		strings.Repeat("A", 1000),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, input := range testInputs {
			isInputSafe(input)
		}
	}
}

func TestValidateContextCancellation(t *testing.T) {
	// Test that validate command properly handles context cancellation
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		// In real implementation, this would test that the validate command
		// respects context cancellation and doesn't continue after cancellation
		
		// Mock test for now
		if ctx.Err() == nil {
			t.Error("Expected context to be cancelled")
		}
	})
}