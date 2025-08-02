package processor

import (
	"fmt"
	"strings"
	"testing"

	"go-toolgit/internal/core/git"
)

func TestMemoryProcessor_FilterFiles(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:    "old",
			Replacement: "new",
		},
	}

	includePatterns := []string{"*.go", "*.js"}
	excludePatterns := []string{"*_test.go", "vendor/*"}

	engine, err := NewReplacementEngine(rules, includePatterns, excludePatterns)
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	processor := &MemoryProcessor{engine: engine}

	files := []git.FileInfo{
		{Path: "main.go", Content: []byte("content")},
		{Path: "main_test.go", Content: []byte("test content")}, // Should be excluded
		{Path: "script.js", Content: []byte("js content")},
		{Path: "README.md", Content: []byte("readme")},     // Should be excluded (no include match)
		{Path: "vendor/lib.go", Content: []byte("vendor")}, // Should be excluded
		{Path: "src/app.go", Content: []byte("app content")},
	}

	filtered := processor.filterFiles(files)

	expectedPaths := []string{"main.go", "script.js", "src/app.go"}
	if len(filtered) != len(expectedPaths) {
		t.Errorf("Expected %d files, got %d", len(expectedPaths), len(filtered))
	}

	for i, file := range filtered {
		if file.Path != expectedPaths[i] {
			t.Errorf("Expected file %q at index %d, got %q", expectedPaths[i], i, file.Path)
		}
	}
}

func TestMemoryProcessor_ProcessContent(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "foo",
			Replacement:   "bar",
			CaseSensitive: true,
		},
		{
			Original:      "hello",
			Replacement:   "hi",
			CaseSensitive: false,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	processor := &MemoryProcessor{engine: engine}

	tests := []struct {
		name            string
		content         string
		expectedContent string
		expectedCount   int
	}{
		{
			name:            "Single replacement",
			content:         "foo world",
			expectedContent: "bar world",
			expectedCount:   1,
		},
		{
			name:            "Multiple replacements",
			content:         "foo hello foo HELLO",
			expectedContent: "bar hi bar hi",
			expectedCount:   4,
		},
		{
			name:            "No replacements",
			content:         "nothing to replace here",
			expectedContent: "nothing to replace here",
			expectedCount:   0,
		},
		{
			name:            "Case insensitive replacement",
			content:         "Hello HELLO hello",
			expectedContent: "hi hi hi",
			expectedCount:   3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, count := processor.processContent(test.content, "test.txt")

			if result != test.expectedContent {
				t.Errorf("Expected content %q, got %q", test.expectedContent, result)
			}

			if count != test.expectedCount {
				t.Errorf("Expected %d replacements, got %d", test.expectedCount, count)
			}
		})
	}
}

func TestMemoryProcessor_ReplaceWholeWords(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "test",
			Replacement:   "exam",
			CaseSensitive: true,
			WholeWord:     true,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	processor := &MemoryProcessor{engine: engine}

	tests := []struct {
		name            string
		content         string
		expectedContent string
		expectedCount   int
	}{
		{
			name:            "Whole word match",
			content:         "test file",
			expectedContent: "exam file",
			expectedCount:   1,
		},
		{
			name:            "Partial word - no match",
			content:         "testing file",
			expectedContent: "testing file",
			expectedCount:   0,
		},
		{
			name:            "Multiple whole words",
			content:         "test this test case",
			expectedContent: "exam this exam case",
			expectedCount:   2,
		},
		{
			name:            "Mixed partial and whole",
			content:         "test testing test",
			expectedContent: "exam testing exam",
			expectedCount:   2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, count := processor.processContent(test.content, "test.txt")

			if result != test.expectedContent {
				t.Errorf("Expected content %q, got %q", test.expectedContent, result)
			}

			if count != test.expectedCount {
				t.Errorf("Expected %d replacements, got %d", test.expectedCount, count)
			}
		})
	}
}

func TestMemoryProcessor_ReplaceWholeWordsInsensitive(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "Test",
			Replacement:   "Exam",
			CaseSensitive: false,
			WholeWord:     true,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	processor := &MemoryProcessor{engine: engine}

	content := "test TEST Test testing"
	expectedContent := "Exam Exam Exam testing"
	expectedCount := 3

	result, count := processor.processContent(content, "test.txt")

	if result != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, result)
	}

	if count != expectedCount {
		t.Errorf("Expected %d replacements, got %d", expectedCount, count)
	}
}

func TestMemoryProcessor_ReplaceAllInsensitive(t *testing.T) {
	processor := &MemoryProcessor{}

	tests := []struct {
		name        string
		content     string
		original    string
		replacement string
		expected    string
	}{
		{
			name:        "Case insensitive replacement",
			content:     "Hello hello HELLO",
			original:    "hello",
			replacement: "hi",
			expected:    "hi hi hi",
		},
		{
			name:        "Mixed case original",
			content:     "Test test TEST",
			original:    "TeSt",
			replacement: "exam",
			expected:    "exam exam exam",
		},
		{
			name:        "No matches",
			content:     "nothing here",
			original:    "missing",
			replacement: "found",
			expected:    "nothing here",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := processor.replaceAllInsensitive(test.content, test.original, test.replacement)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkMemoryProcessor_ProcessContent(b *testing.B) {
	rules := []ReplacementRule{
		{Original: "foo", Replacement: "bar"},
		{Original: "hello", Replacement: "hi"},
		{Original: "world", Replacement: "universe"},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		b.Fatalf("Failed to create replacement engine: %v", err)
	}

	processor := &MemoryProcessor{engine: engine}
	content := strings.Repeat("foo hello world ", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.processContent(content, "test.txt")
	}
}

func BenchmarkMemoryProcessor_FilterFiles(b *testing.B) {
	rules := []ReplacementRule{{Original: "old", Replacement: "new"}}
	engine, _ := NewReplacementEngine(rules, []string{"*.go"}, []string{"*_test.go"})
	processor := &MemoryProcessor{engine: engine}

	files := make([]git.FileInfo, 1000)
	for i := 0; i < 1000; i++ {
		files[i] = git.FileInfo{
			Path:    fmt.Sprintf("file%d.go", i),
			Content: []byte("content"),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.filterFiles(files)
	}
}
