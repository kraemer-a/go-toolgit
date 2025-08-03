package github

import (
	"context"
	"strings"
	"testing"
	"time"

	"go-toolgit/internal/core/security"
)

func TestSecureClient_ValidatesInputs(t *testing.T) {
	// Create a secure client (this will fail with invalid config, but that's expected)
	config := &Config{
		BaseURL:    "https://api.github.com",
		Token:      "ghp_valid_test_token_123456789",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
	
	secureClient, err := NewSecureClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure client: %v", err)
	}
	
	ctx := context.Background()
	
	// Test that malicious search options are rejected
	maliciousSearchOptions := SearchOptions{
		Query: "test; rm -rf /",
		Owner: "<script>alert('xss')</script>",
	}
	
	_, err = secureClient.SearchRepositories(ctx, maliciousSearchOptions)
	if err == nil {
		t.Error("Expected secure client to reject malicious search options")
	}
	
	if !strings.Contains(err.Error(), "invalid search options") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func TestSecureClient_ValidatesTeamAccess(t *testing.T) {
	config := &Config{
		BaseURL:    "https://api.github.com",
		Token:      "ghp_valid_test_token_123456789",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
	
	secureClient, err := NewSecureClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure client: %v", err)
	}
	
	ctx := context.Background()
	
	// Test that malicious organization/team names are rejected
	_, err = secureClient.GetTeam(ctx, "org; rm -rf /", "team")
	if err == nil {
		t.Error("Expected secure client to reject malicious organization name")
	}
	
	if !strings.Contains(err.Error(), "invalid organization") {
		t.Errorf("Expected organization validation error, got: %v", err)
	}
}

func TestSecureClient_ValidatesPullRequestOptions(t *testing.T) {
	config := &Config{
		BaseURL:    "https://api.github.com",
		Token:      "ghp_valid_test_token_123456789",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
	
	secureClient, err := NewSecureClient(config)
	if err != nil {
		t.Fatalf("Failed to create secure client: %v", err)
	}
	
	ctx := context.Background()
	
	// Test that malicious PR options are rejected
	maliciousPR := &PullRequestOptions{
		Title: "<script>alert('xss')</script>",
		Head:  "feature; rm -rf /",
		Base:  "main",
		Body:  "Test PR",
	}
	
	_, err = secureClient.CreatePullRequest(ctx, "owner", "repo", maliciousPR)
	if err == nil {
		t.Error("Expected secure client to reject malicious PR options")
	}
	
	if !strings.Contains(err.Error(), "invalid pull request options") {
		t.Errorf("Expected PR validation error, got: %v", err)
	}
}

func TestSecureClient_RejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		desc   string
	}{
		{
			name: "malicious token",
			config: Config{
				BaseURL:    "https://api.github.com",
				Token:      "token; rm -rf /",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			desc: "Should reject tokens with command injection",
		},
		{
			name: "malicious URL",
			config: Config{
				BaseURL:    "https://api.github.com<script>alert('xss')</script>",
				Token:      "ghp_valid_token_123456789",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			desc: "Should reject URLs with script injection",
		},
		{
			name: "invalid timeout",
			config: Config{
				BaseURL:    "https://api.github.com",
				Token:      "ghp_valid_token_123456789",
				Timeout:    -1 * time.Second,
				MaxRetries: 3,
			},
			desc: "Should reject negative timeouts",
		},
		{
			name: "excessive retries",
			config: Config{
				BaseURL:    "https://api.github.com",
				Token:      "ghp_valid_token_123456789",
				Timeout:    30 * time.Second,
				MaxRetries: 1000,
			},
			desc: "Should reject excessive retry counts",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSecureClient(&tt.config)
			if err == nil {
				t.Errorf("Expected NewSecureClient to reject config: %s", tt.desc)
			}
		})
	}
}

func TestBuildSecureSearchQuery(t *testing.T) {
	validator := security.NewInputValidator(true)
	
	tests := []struct {
		name        string
		opts        SearchOptions
		expectError bool
		desc        string
	}{
		{
			name: "valid search options",
			opts: SearchOptions{
				Query:    "machine learning",
				Owner:    "microsoft",
				Language: "python",
			},
			expectError: false,
			desc:        "Should accept valid search options",
		},
		{
			name: "malicious query",
			opts: SearchOptions{
				Query: "test; rm -rf /",
			},
			expectError: true,
			desc:        "Should reject command injection in query",
		},
		{
			name: "malicious owner",
			opts: SearchOptions{
				Owner: "<script>alert('xss')</script>",
			},
			expectError: true,
			desc:        "Should reject script injection in owner",
		},
		{
			name: "extremely long query",
			opts: SearchOptions{
				Query: strings.Repeat("A", 10000),
			},
			expectError: true,
			desc:        "Should reject excessively long queries",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildSecureSearchQuery(tt.opts, validator)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s", tt.desc)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.desc, err)
				}
			}
		})
	}
}

func TestSecureClient_SanitizesInput(t *testing.T) {
	validator := security.NewInputValidator(true)
	
	// Test that input sanitization removes dangerous characters
	maliciousInput := "test\x00\x01input"
	sanitized := validator.SanitizeString(maliciousInput)
	
	if strings.Contains(sanitized, "\x00") || strings.Contains(sanitized, "\x01") {
		t.Error("Sanitization failed to remove dangerous characters")
	}
	
	if sanitized != "testinput" {
		t.Errorf("Expected 'testinput', got '%s'", sanitized)
	}
}

// Benchmark tests for security overhead
func BenchmarkSecureSearchQuery(b *testing.B) {
	validator := security.NewInputValidator(true)
	opts := SearchOptions{
		Query:    "machine learning",
		Owner:    "microsoft",
		Language: "python",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildSecureSearchQuery(opts, validator)
	}
}

func BenchmarkUnsafeSearchQuery(b *testing.B) {
	opts := SearchOptions{
		Query:    "machine learning",
		Owner:    "microsoft",
		Language: "python",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildSearchQuery(opts)
	}
}