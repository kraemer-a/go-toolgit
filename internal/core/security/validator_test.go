package security

import (
	"strings"
	"testing"
)

func TestInputValidator_ValidateString(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name      string
		field     string
		value     string
		maxLength int
		wantError bool
	}{
		{
			name:      "valid string",
			field:     "test",
			value:     "hello world",
			maxLength: 100,
			wantError: false,
		},
		{
			name:      "string too long",
			field:     "test",
			value:     strings.Repeat("a", 101),
			maxLength: 100,
			wantError: true,
		},
		{
			name:      "command injection",
			field:     "test",
			value:     "hello; rm -rf /",
			maxLength: 100,
			wantError: true,
		},
		{
			name:      "script injection",
			field:     "test",
			value:     "<script>alert('xss')</script>",
			maxLength: 100,
			wantError: true,
		},
		{
			name:      "path traversal",
			field:     "test",
			value:     "../../../etc/passwd",
			maxLength: 100,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateString(tt.field, tt.value, tt.maxLength)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateString() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_ValidateToken(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name      string
		field     string
		token     string
		wantError bool
	}{
		{
			name:      "valid GitHub token",
			field:     "token",
			token:     "ghp_1234567890abcdef123456789",
			wantError: false,
		},
		{
			name:      "empty token",
			field:     "token",
			token:     "",
			wantError: true,
		},
		{
			name:      "token with command injection",
			field:     "token",
			token:     "ghp_token; rm -rf /",
			wantError: true,
		},
		{
			name:      "token too short",
			field:     "token",
			token:     "ghp_short",
			wantError: true,
		},
		{
			name:      "extremely long token",
			field:     "token",
			token:     strings.Repeat("a", MaxTokenLength+1),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateToken(tt.field, tt.token)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateToken() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_ValidateURL(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name      string
		field     string
		url       string
		wantError bool
	}{
		{
			name:      "valid HTTPS URL",
			field:     "url",
			url:       "https://api.github.com",
			wantError: false,
		},
		{
			name:      "valid HTTP URL",
			field:     "url",
			url:       "http://localhost:8080",
			wantError: false,
		},
		{
			name:      "empty URL",
			field:     "url",
			url:       "",
			wantError: true,
		},
		{
			name:      "invalid scheme",
			field:     "url",
			url:       "ftp://example.com",
			wantError: true,
		},
		{
			name:      "path traversal in URL",
			field:     "url",
			url:       "https://example.com/../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "script injection in URL",
			field:     "url",
			url:       "https://example.com/<script>alert('xss')</script>",
			wantError: true,
		},
		{
			name:      "malformed URL",
			field:     "url",
			url:       "not-a-url",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.field, tt.url)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateURL() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_ValidateFilePath(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name      string
		field     string
		path      string
		wantError bool
	}{
		{
			name:      "valid relative path",
			field:     "path",
			path:      "src/main.go",
			wantError: false,
		},
		{
			name:      "valid temp path",
			field:     "path",
			path:      "/tmp/test-repo",
			wantError: false,
		},
		{
			name:      "empty path",
			field:     "path",
			path:      "",
			wantError: true,
		},
		{
			name:      "path traversal",
			field:     "path",
			path:      "../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "dangerous absolute path",
			field:     "path",
			path:      "/etc/shadow",
			wantError: true,
		},
		{
			name:      "windows system path",
			field:     "path",
			path:      "C:\\Windows\\System32",
			wantError: true,
		},
		{
			name:      "command injection in path",
			field:     "path",
			path:      "/tmp/test; rm -rf /",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateFilePath(tt.field, tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateFilePath() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_ValidateBranchName(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name       string
		field      string
		branchName string
		wantError  bool
	}{
		{
			name:       "valid branch name",
			field:     "branch",
			branchName: "feature-123",
			wantError:  false,
		},
		{
			name:       "valid branch with slash",
			field:     "branch", 
			branchName: "feature/user-auth",
			wantError:  false,
		},
		{
			name:       "empty branch name",
			field:     "branch",
			branchName: "",
			wantError:  true,
		},
		{
			name:       "branch with space",
			field:     "branch",
			branchName: "feature branch",
			wantError:  true,
		},
		{
			name:       "branch with invalid characters",
			field:     "branch",
			branchName: "feature~123",
			wantError:  true,
		},
		{
			name:       "branch starting with slash",
			field:     "branch",
			branchName: "/feature",
			wantError:  true,
		},
		{
			name:       "branch ending with slash",
			field:     "branch",
			branchName: "feature/",
			wantError:  true,
		},
		{
			name:       "branch starting with dot",
			field:     "branch",
			branchName: ".feature",
			wantError:  true,
		},
		{
			name:       "command injection in branch",
			field:     "branch",
			branchName: "feature; rm -rf /",
			wantError:  true,
		},
		{
			name:       "extremely long branch name",
			field:     "branch",
			branchName: strings.Repeat("a", MaxBranchNameLength+1),
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateBranchName(tt.field, tt.branchName)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateBranchName() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_ValidateSearchQuery(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name      string
		field     string
		query     string
		wantError bool
	}{
		{
			name:      "valid search query",
			field:     "query",
			query:     "machine learning python",
			wantError: false,
		},
		{
			name:      "query with special chars",
			field:     "query",
			query:     "C++ OR Java",
			wantError: false,
		},
		{
			name:      "command injection in query",
			field:     "query",
			query:     "test; rm -rf /",
			wantError: true,
		},
		{
			name:      "script injection in query",
			field:     "query",
			query:     "<script>alert('xss')</script>",
			wantError: true,
		},
		{
			name:      "extremely long query",
			field:     "query",
			query:     strings.Repeat("a", MaxSearchQueryLength+1),
			wantError: true,
		},
		{
			name:      "ReDoS pattern",
			field:     "query",
			query:     strings.Repeat("(a+)+", 10),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSearchQuery(tt.field, tt.query)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSearchQuery() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_ValidateReplacement(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name        string
		field       string
		replacement string
		wantError   bool
	}{
		{
			name:        "valid replacement",
			field:       "replacement",
			replacement: "newValue",
			wantError:   false,
		},
		{
			name:        "replacement with newlines",
			field:       "replacement",
			replacement: "line1\nline2",
			wantError:   false,
		},
		{
			name:        "command injection in replacement",
			field:       "replacement",
			replacement: "value; rm -rf /",
			wantError:   true,
		},
		{
			name:        "script injection in replacement",
			field:       "replacement",
			replacement: "<script>alert('xss')</script>",
			wantError:   true,
		},
		{
			name:        "extremely long replacement",
			field:       "replacement",
			replacement: strings.Repeat("a", MaxReplacementLength+1),
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateReplacement(tt.field, tt.replacement)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateReplacement() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestInputValidator_SanitizeString(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "string with null bytes",
			input:    "hello\x00world",
			expected: "helloworld",
		},
		{
			name:     "string with control characters",
			input:    "hello\x01\x02world",
			expected: "helloworld",
		},
		{
			name:     "string with valid whitespace",
			input:    "hello\t\n\rworld",
			expected: "hello\t\n\rworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeString() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestInputValidator_StrictMode(t *testing.T) {
	strictValidator := NewInputValidator(true)
	lenientValidator := NewInputValidator(false)
	
	sqlInjectionInput := "test' OR '1'='1"
	
	// Strict mode should reject SQL injection patterns
	err := strictValidator.ValidateString("test", sqlInjectionInput, 100)
	if err == nil {
		t.Error("Expected strict validator to reject SQL injection pattern")
	}
	
	// Lenient mode should allow it
	err = lenientValidator.ValidateString("test", sqlInjectionInput, 100)
	if err != nil {
		t.Errorf("Expected lenient validator to allow SQL pattern, got error: %v", err)
	}
}

func TestInputValidator_IsAllowedFileExtension(t *testing.T) {
	validator := NewInputValidator(false)
	
	tests := []struct {
		name              string
		filename          string
		allowedExtensions []string
		expected          bool
	}{
		{
			name:              "allowed extension",
			filename:          "main.go",
			allowedExtensions: []string{".go", ".java"},
			expected:          true,
		},
		{
			name:              "disallowed extension",
			filename:          "script.sh",
			allowedExtensions: []string{".go", ".java"},
			expected:          false,
		},
		{
			name:              "no restrictions",
			filename:          "anything.xyz",
			allowedExtensions: []string{},
			expected:          true,
		},
		{
			name:              "case insensitive",
			filename:          "Main.GO",
			allowedExtensions: []string{".go"},
			expected:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.IsAllowedFileExtension(tt.filename, tt.allowedExtensions)
			if result != tt.expected {
				t.Errorf("IsAllowedFileExtension() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateString(b *testing.B) {
	validator := NewInputValidator(false)
	testString := "this is a normal string for validation"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateString("test", testString, 100)
	}
}

func BenchmarkValidateToken(b *testing.B) {
	validator := NewInputValidator(false)
	testToken := "ghp_1234567890abcdef1234567890"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateToken("token", testToken)
	}
}

func BenchmarkSanitizeString(b *testing.B) {
	validator := NewInputValidator(false)
	testString := "hello\x00\x01world\x02test"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.SanitizeString(testString)
	}
}