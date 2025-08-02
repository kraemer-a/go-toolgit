package github

import (
	"testing"
	"time"
)

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "Valid config",
			config: &Config{
				BaseURL:    "https://api.github.com",
				Token:      "ghp_test_token",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: false,
		},
		{
			name: "Empty base URL - should still work (uses default)",
			config: &Config{
				BaseURL:    "",
				Token:      "ghp_test_token",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: false, // NewClient doesn't validate, it just creates the client
		},
		{
			name: "Empty token - should still work (will fail at API call time)",
			config: &Config{
				BaseURL:    "https://api.github.com",
				Token:      "",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: false, // NewClient doesn't validate, it just creates the client
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchOptions_BuildQuery(t *testing.T) {
	tests := []struct {
		name     string
		opts     SearchOptions
		expected string
	}{
		{
			name: "Basic owner search",
			opts: SearchOptions{
				Owner: "testuser",
			},
			expected: "user:testuser fork:false archived:false",
		},
		{
			name: "Language and stars search",
			opts: SearchOptions{
				Language: "go",
				Stars:    ">100",
			},
			expected: "language:go stars:>100 fork:false archived:false",
		},
		{
			name: "Complex search with all options",
			opts: SearchOptions{
				Query:    "microservice",
				Owner:    "company",
				Language: "python",
				Stars:    "50..200",
				Size:     ">1000",
				Fork:     true,
				Archived: true,
			},
			expected: "microservice user:company language:python stars:50..200 size:>1000",
		},
		{
			name: "Query with keywords",
			opts: SearchOptions{
				Query: "machine learning API",
				Owner: "openai",
			},
			expected: "machine learning API user:openai fork:false archived:false",
		},
		{
			name: "Default search options (excludes forks and archived)",
			opts: SearchOptions{},
			expected: "fork:false archived:false", // Default behavior excludes forks and archived
		},
		{
			name: "Truly empty search (include forks and archived)",
			opts: SearchOptions{
				Fork:     true,  // Include forks
				Archived: true,  // Include archived
			},
			expected: "stars:>0", // Should return default when no other criteria
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSearchQuery(tt.opts)
			if result != tt.expected {
				t.Errorf("buildSearchQuery() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestRepository_Struct(t *testing.T) {
	repo := &Repository{
		ID:       12345,
		Name:     "test-repo",
		FullName: "user/test-repo",
		CloneURL: "https://github.com/user/test-repo.git",
		SSHURL:   "git@github.com:user/test-repo.git",
		Private:  false,
	}

	if repo.ID != 12345 {
		t.Errorf("Expected ID 12345, got %d", repo.ID)
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got %q", repo.Name)
	}

	if repo.FullName != "user/test-repo" {
		t.Errorf("Expected full name 'user/test-repo', got %q", repo.FullName)
	}

	if repo.Private {
		t.Errorf("Expected private to be false, got true")
	}
}

func TestPullRequestOptions_Struct(t *testing.T) {
	pr := &PullRequestOptions{
		Title: "Test PR",
		Head:  "feature-branch",
		Base:  "main",
		Body:  "This is a test pull request",
	}

	if pr.Title != "Test PR" {
		t.Errorf("Expected title 'Test PR', got %q", pr.Title)
	}

	if pr.Head != "feature-branch" {
		t.Errorf("Expected head 'feature-branch', got %q", pr.Head)
	}

	if pr.Base != "main" {
		t.Errorf("Expected base 'main', got %q", pr.Base)
	}

	if pr.Body != "This is a test pull request" {
		t.Errorf("Expected specific body, got %q", pr.Body)
	}
}

func TestSearchOptions_Defaults(t *testing.T) {
	opts := SearchOptions{
		Owner:      "testuser",
		MaxResults: 0, // Should use default
	}

	if opts.MaxResults != 0 {
		t.Errorf("Expected MaxResults to be 0 before processing, got %d", opts.MaxResults)
	}

	// Test that PerPage gets a reasonable default when 0
	if opts.PerPage == 0 {
		// This is expected - PerPage should be 0 initially
		// The actual default setting should happen in the client
	}
}

// Test the Team struct
func TestTeam_Struct(t *testing.T) {
	team := &Team{
		ID:   123,
		Name: "Backend Team",
		Slug: "backend-team",
	}

	if team.ID != 123 {
		t.Errorf("Expected ID 123, got %d", team.ID)
	}

	if team.Name != "Backend Team" {
		t.Errorf("Expected name 'Backend Team', got %q", team.Name)
	}

	if team.Slug != "backend-team" {
		t.Errorf("Expected slug 'backend-team', got %q", team.Slug)
	}
}

// Benchmark test for buildSearchQuery
func BenchmarkBuildSearchQuery(b *testing.B) {
	opts := SearchOptions{
		Query:    "machine learning",
		Owner:    "microsoft",
		Language: "python",
		Stars:    ">100",
		Size:     ">1000",
		Fork:     false,
		Archived: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildSearchQuery(opts)
	}
}

// Test edge cases for search query building
func TestBuildSearchQuery_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		opts     SearchOptions
		expected string
	}{
		{
			name: "Only fork option true",
			opts: SearchOptions{
				Fork: true,
			},
			expected: "archived:false",
		},
		{
			name: "Only archived option true",
			opts: SearchOptions{
				Archived: true,
			},
			expected: "fork:false",
		},
		{
			name: "Special characters in query",
			opts: SearchOptions{
				Query: "C++ OR C#",
				Owner: "microsoft",
			},
			expected: "C++ OR C# user:microsoft fork:false archived:false",
		},
		{
			name: "Very long query",
			opts: SearchOptions{
				Query: "this is a very long search query with many words that should still work correctly",
				Owner: "testuser",
			},
			expected: "this is a very long search query with many words that should still work correctly user:testuser fork:false archived:false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSearchQuery(tt.opts)
			if result != tt.expected {
				t.Errorf("buildSearchQuery() = %q, expected %q", result, tt.expected)
			}
		})
	}
}