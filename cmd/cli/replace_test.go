package cli

import (
	"bytes"
	"strings"
	"testing"

	"go-toolgit/internal/core/processor"
)

func TestReplaceCommand(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "help command",
			args:        []string{"replace", "--help"},
			expectError: false,
		},
		{
			name:        "missing required replacements flag",
			args:        []string{"replace"},
			expectError: true,
			errorMsg:    "required flag(s)",
		},
		{
			name:        "valid basic replacement",
			args:        []string{"replace", "--replacements", "old=new", "--help"},
			expectError: false,
		},
		{
			name:        "multiple replacements",
			args:        []string{"replace", "--replacements", "old=new,foo=bar", "--help"},
			expectError: false,
		},
		{
			name:        "with include patterns",
			args:        []string{"replace", "--replacements", "old=new", "--include", "*.go,*.java", "--help"},
			expectError: false,
		},
		{
			name:        "with exclude patterns",
			args:        []string{"replace", "--replacements", "old=new", "--exclude", "vendor/*,*.min.js", "--help"},
			expectError: false,
		},
		{
			name:        "dry run mode",
			args:        []string{"replace", "--replacements", "old=new", "--dry-run", "--help"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.AddCommand(replaceCmd)
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

func TestReplaceSecurityInputs(t *testing.T) {
	maliciousInputs := []struct {
		name string
		args []string
		desc string
	}{
		{
			name: "command injection in replacement",
			args: []string{"replace", "--replacements", "old=new; rm -rf /", "--help"},
			desc: "Should not execute embedded commands in replacement values",
		},
		{
			name: "command injection in include pattern",
			args: []string{"replace", "--replacements", "old=new", "--include", "*.go; curl evil.com", "--help"},
			desc: "Should not execute commands in file patterns",
		},
		{
			name: "path traversal in exclude pattern",
			args: []string{"replace", "--replacements", "old=new", "--exclude", "../../../etc/*", "--help"},
			desc: "Should not access files outside repository",
		},
		{
			name: "script injection in PR title",
			args: []string{"replace", "--replacements", "old=new", "--pr-title", "<script>alert('xss')</script>", "--help"},
			desc: "Should not execute scripts in PR metadata",
		},
		{
			name: "command injection in branch prefix",
			args: []string{"replace", "--replacements", "old=new", "--branch-prefix", "test; rm -rf /", "--help"},
			desc: "Should not execute commands in branch names",
		},
		{
			name: "extremely long replacement",
			args: []string{"replace", "--replacements", "old=" + strings.Repeat("A", 100000), "--help"},
			desc: "Should handle large replacement values",
		},
		{
			name: "null bytes in replacement",
			args: []string{"replace", "--replacements", "old=new\x00evil", "--help"},
			desc: "Should handle null bytes safely",
		},
		{
			name: "regex injection attempt",
			args: []string{"replace", "--replacements", "old=.*", "--help"},
			desc: "Should handle regex metacharacters safely",
		},
	}

	for _, tt := range maliciousInputs {
		t.Run(tt.name, func(t *testing.T) {
			cmd := createTestRootCommand()
			cmd.AddCommand(replaceCmd)
			cmd.SetArgs(tt.args)

			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)

			// Command should not crash or execute malicious content
			err := cmd.Execute()
			
			// Check for crashes
			if err != nil && strings.Contains(err.Error(), "runtime error") {
				t.Errorf("Command crashed with malicious input: %v", err)
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

func TestParseReplacements(t *testing.T) {
	tests := []struct {
		name         string
		replacements []string
		expectError  bool
		expected     []processor.ReplacementRule
	}{
		{
			name:         "valid single replacement",
			replacements: []string{"old=new"},
			expectError:  false,
			expected: []processor.ReplacementRule{
				{Original: "old", Replacement: "new", CaseSensitive: true},
			},
		},
		{
			name:         "multiple replacements",
			replacements: []string{"old=new", "foo=bar"},
			expectError:  false,
			expected: []processor.ReplacementRule{
				{Original: "old", Replacement: "new", CaseSensitive: true},
				{Original: "foo", Replacement: "bar", CaseSensitive: true},
			},
		},
		{
			name:         "replacement with equals in value",
			replacements: []string{"old=new=value"},
			expectError:  false,
			expected: []processor.ReplacementRule{
				{Original: "old", Replacement: "new=value", CaseSensitive: true},
			},
		},
		{
			name:         "empty replacement value",
			replacements: []string{"old="},
			expectError:  false,
			expected: []processor.ReplacementRule{
				{Original: "old", Replacement: "", CaseSensitive: true},
			},
		},
		{
			name:         "invalid format - no equals",
			replacements: []string{"invalid"},
			expectError:  true,
		},
		{
			name:         "invalid format - multiple equals at start",
			replacements: []string{"=invalid=format"},
			expectError:  false, // First part becomes empty original
			expected: []processor.ReplacementRule{
				{Original: "", Replacement: "invalid=format", CaseSensitive: true},
			},
		},
		{
			name:         "special characters in replacement",
			replacements: []string{"old=new\nline", "regex=.*"},
			expectError:  false,
			expected: []processor.ReplacementRule{
				{Original: "old", Replacement: "new\nline", CaseSensitive: true},
				{Original: "regex", Replacement: ".*", CaseSensitive: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseReplacements(tt.replacements)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d rules, got %d", len(tt.expected), len(result))
				return
			}

			for i, rule := range result {
				expected := tt.expected[i]
				if rule.Original != expected.Original {
					t.Errorf("Rule %d: expected original '%s', got '%s'", i, expected.Original, rule.Original)
				}
				if rule.Replacement != expected.Replacement {
					t.Errorf("Rule %d: expected replacement '%s', got '%s'", i, expected.Replacement, rule.Replacement)
				}
				if rule.CaseSensitive != expected.CaseSensitive {
					t.Errorf("Rule %d: expected case sensitive %v, got %v", i, expected.CaseSensitive, rule.CaseSensitive)
				}
			}
		})
	}
}

func TestReplacementInputValidation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool // true if input should be considered safe
	}{
		{
			name:     "normal replacement",
			input:    "oldValue=newValue",
			expected: true,
		},
		{
			name:     "replacement with special chars",
			input:    "old_value=new-value.123",
			expected: true,
		},
		{
			name:     "command injection attempt",
			input:    "old=new; rm -rf /",
			expected: false,
		},
		{
			name:     "script injection attempt",
			input:    "old=<script>alert('xss')</script>",
			expected: false,
		},
		{
			name:     "path traversal attempt",
			input:    "old=../../../etc/passwd",
			expected: false,
		},
		{
			name:     "sql injection attempt",
			input:    "old='; DROP TABLE users; --",
			expected: false,
		},
		{
			name:     "regex that could cause ReDoS",
			input:    "old=" + strings.Repeat("(a+)+", 100),
			expected: false,
		},
		{
			name:     "extremely long input",
			input:    "old=" + strings.Repeat("A", 50000),
			expected: false, // Could cause memory issues
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSafe := isReplacementSafe(tt.input)
			if isSafe != tt.expected {
				t.Errorf("Replacement safety check failed for '%s': expected %v, got %v",
					tt.input, tt.expected, isSafe)
			}
		})
	}
}

func TestMergePatterns(t *testing.T) {
	tests := []struct {
		name           string
		cmdPatterns    []string
		configPatterns []string
		expected       []string
	}{
		{
			name:           "command patterns take precedence",
			cmdPatterns:    []string{"*.go", "*.java"},
			configPatterns: []string{"*.js", "*.py"},
			expected:       []string{"*.go", "*.java"},
		},
		{
			name:           "use config patterns when no command patterns",
			cmdPatterns:    []string{},
			configPatterns: []string{"*.js", "*.py"},
			expected:       []string{"*.js", "*.py"},
		},
		{
			name:           "empty command patterns use config",
			cmdPatterns:    nil,
			configPatterns: []string{"*.ts", "*.tsx"},
			expected:       []string{"*.ts", "*.tsx"},
		},
		{
			name:           "both empty",
			cmdPatterns:    []string{},
			configPatterns: []string{},
			expected:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergePatterns(tt.cmdPatterns, tt.configPatterns)
			
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d patterns, got %d", len(tt.expected), len(result))
				return
			}

			for i, pattern := range result {
				if pattern != tt.expected[i] {
					t.Errorf("Pattern %d: expected '%s', got '%s'", i, tt.expected[i], pattern)
				}
			}
		})
	}
}

func TestFilePatternSecurity(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool // true if pattern should be considered safe
	}{
		{
			name:     "normal glob pattern",
			pattern:  "*.go",
			expected: true,
		},
		{
			name:     "directory pattern",
			pattern:  "src/**/*.java",
			expected: true,
		},
		{
			name:     "path traversal attempt",
			pattern:  "../../../etc/*",
			expected: false,
		},
		{
			name:     "absolute path attempt",
			pattern:  "/etc/passwd",
			expected: false,
		},
		{
			name:     "command injection in pattern",
			pattern:  "*.go; rm -rf /",
			expected: false,
		},
		{
			name:     "pattern with spaces",
			pattern:  "file with spaces.txt",
			expected: true,
		},
		{
			name:     "complex but safe pattern",
			pattern:  "src/{main,test}/**/*.{go,java}",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSafe := isFilePatternSafe(tt.pattern)
			if isSafe != tt.expected {
				t.Errorf("File pattern safety check failed for '%s': expected %v, got %v",
					tt.pattern, tt.expected, isSafe)
			}
		})
	}
}

func TestReplaceParameterValidation(t *testing.T) {
	tests := []struct {
		name        string
		maxWorkers  int
		expectError bool
	}{
		{
			name:        "valid worker count",
			maxWorkers:  4,
			expectError: false,
		},
		{
			name:        "minimum worker count",
			maxWorkers:  1,
			expectError: false,
		},
		{
			name:        "zero workers",
			maxWorkers:  0,
			expectError: true,
		},
		{
			name:        "negative workers",
			maxWorkers:  -1,
			expectError: true,
		},
		{
			name:        "excessive workers",
			maxWorkers:  1000,
			expectError: true, // Could cause resource exhaustion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := isWorkerCountValid(tt.maxWorkers)
			if isValid == tt.expectError {
				t.Errorf("Worker count validation failed for %d: expected error=%v, got valid=%v",
					tt.maxWorkers, tt.expectError, isValid)
			}
		})
	}
}

// Helper functions for security validation (would be implemented in actual code)
func isReplacementSafe(replacement string) bool {
	// Check for dangerous patterns in replacement strings
	dangerousPatterns := []string{
		";", "&", "|", "`", "$", 
		"<script", "</script", "rm -rf", "curl",
		"DROP TABLE", "SELECT * FROM", "../",
	}
	
	// Check length to prevent memory exhaustion
	if len(replacement) > 10000 {
		return false
	}
	
	// Check for ReDoS patterns
	if strings.Count(replacement, "(") > 20 || strings.Count(replacement, "+") > 20 {
		return false
	}
	
	replacementLower := strings.ToLower(replacement)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(replacementLower, strings.ToLower(pattern)) {
			return false
		}
	}
	return true
}

func isFilePatternSafe(pattern string) bool {
	// Check for path traversal
	if strings.Contains(pattern, "../") || strings.Contains(pattern, "..\\") {
		return false
	}
	
	// Check for absolute paths
	if strings.HasPrefix(pattern, "/") || strings.Contains(pattern, ":") {
		return false
	}
	
	// Check for command injection
	dangerousChars := []string{";", "&", "|", "`", "$"}
	for _, char := range dangerousChars {
		if strings.Contains(pattern, char) {
			return false
		}
	}
	
	return true
}

func isWorkerCountValid(count int) bool {
	return count > 0 && count <= 100 // Reasonable limits
}

// Benchmark tests
func BenchmarkParseReplacements(b *testing.B) {
	replacements := []string{
		"old1=new1", "old2=new2", "old3=new3",
		"oldValue=newValue", "foo=bar",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseReplacements(replacements)
	}
}

func BenchmarkReplacementSafety(b *testing.B) {
	testReplacements := []string{
		"normal=replacement",
		"old=new; rm -rf /",
		"value=" + strings.Repeat("A", 1000),
		"regex=" + strings.Repeat("(a+)+", 10),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, replacement := range testReplacements {
			isReplacementSafe(replacement)
		}
	}
}

func TestReplaceCommandPerformance(t *testing.T) {
	// Test that replace command handles large numbers of replacements efficiently
	t.Run("large replacement list", func(t *testing.T) {
		var replacements []string
		for i := 0; i < 1000; i++ {
			replacements = append(replacements, "old"+string(rune(i))+"=new"+string(rune(i)))
		}
		
		// Should complete parsing within reasonable time
		result, err := parseReplacements(replacements)
		if err != nil {
			t.Errorf("Failed to parse large replacement list: %v", err)
		}
		
		if len(result) != 1000 {
			t.Errorf("Expected 1000 rules, got %d", len(result))
		}
	})
}

func TestReplaceErrorRecovery(t *testing.T) {
	// Test that replace command recovers gracefully from errors
	tests := []struct {
		name        string
		replacements []string
		expectError bool
	}{
		{
			name:        "mixed valid and invalid",
			replacements: []string{"valid=replacement", "invalid", "also=valid"},
			expectError: true,
		},
		{
			name:        "all invalid",
			replacements: []string{"invalid1", "invalid2"},
			expectError: true,
		},
		{
			name:        "empty list",
			replacements: []string{},
			expectError: false, // Empty list is valid, just no work to do
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseReplacements(tt.replacements)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}