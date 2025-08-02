package processor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplacementEngine_BasicReplacement(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "hello",
			Replacement:   "world",
			Regex:         false,
			CaseSensitive: true,
			WholeWord:     false,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	content := "hello world hello"
	expected := "world world world"

	result := engine.replaceContent(content)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplacementEngine_CaseInsensitive(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "Hello",
			Replacement:   "Hi",
			Regex:         false,
			CaseSensitive: false,
			WholeWord:     false,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	content := "hello HELLO Hello"
	expected := "Hi Hi Hi"

	result := engine.replaceContent(content)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplacementEngine_WholeWord(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "test",
			Replacement:   "exam",
			Regex:         false,
			CaseSensitive: true,
			WholeWord:     true,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	content := "test testing testable"
	expected := "exam testing testable" // Only "test" as whole word should be replaced

	result := engine.replaceContent(content)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplacementEngine_RegexReplacement(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      `\d+`,
			Replacement:   "NUMBER",
			Regex:         true,
			CaseSensitive: true,
			WholeWord:     false,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	content := "There are 123 items and 456 more"
	expected := "There are NUMBER items and NUMBER more"

	result := engine.replaceContent(content)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplacementEngine_MultipleRules(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "foo",
			Replacement:   "bar",
			Regex:         false,
			CaseSensitive: true,
			WholeWord:     false,
		},
		{
			Original:      "123",
			Replacement:   "456",
			Regex:         false,
			CaseSensitive: true,
			WholeWord:     false,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	content := "foo 123 foo"
	expected := "bar 456 bar"

	result := engine.replaceContent(content)
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestReplacementEngine_InvalidRegex(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "[invalid",
			Replacement:   "valid",
			Regex:         true,
			CaseSensitive: true,
			WholeWord:     false,
		},
	}

	_, err := NewReplacementEngine(rules, []string{}, []string{})
	if err == nil {
		t.Error("Expected error for invalid regex, got nil")
	}
}

func TestReplacementEngine_ShouldProcessFile(t *testing.T) {
	includePatterns := []string{"*.go", "*.js"}
	excludePatterns := []string{"*_test.go", "vendor/*"}

	engine, err := NewReplacementEngine([]ReplacementRule{}, includePatterns, excludePatterns)
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},          // Matches include pattern
		{"script.js", true},        // Matches include pattern
		{"main_test.go", false},    // Matches exclude pattern
		{"vendor/lib.go", false},   // Matches exclude pattern
		{"README.md", false},       // Doesn't match include pattern
		{"src/app.go", true},       // Matches include pattern
	}

	for _, test := range tests {
		result := engine.shouldProcessFile(test.path)
		if result != test.expected {
			t.Errorf("shouldProcessFile(%q) = %v, expected %v", test.path, result, test.expected)
		}
	}
}

func TestReplacementEngine_IsBinaryFile(t *testing.T) {
	engine, err := NewReplacementEngine([]ReplacementRule{}, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	// Create temporary test files
	tmpDir := t.TempDir()
	
	// Create a text file
	textFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(textFile, []byte("Hello, this is text content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}

	// Create a binary file (with null bytes)
	binaryFile := filepath.Join(tmpDir, "test.bin")
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	err = os.WriteFile(binaryFile, binaryContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create binary file: %v", err)
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{textFile, false},   // Text file should not be binary
		{binaryFile, true},  // Binary file should be binary
		{"nonexistent.txt", true}, // Non-existent files are treated as binary (safe default)
	}

	for _, test := range tests {
		result := engine.isBinaryFile(test.path)
		if result != test.expected {
			t.Errorf("isBinaryFile(%q) = %v, expected %v", test.path, result, test.expected)
		}
	}
}

// Helper method to add to ReplacementEngine for testing
func (e *ReplacementEngine) replaceContent(content string) string {
	result := content
	for _, rule := range e.rules {
		if rule.Regex && rule.compiled != nil {
			result = rule.compiled.ReplaceAllString(result, rule.Replacement)
		} else {
			if rule.CaseSensitive {
				if rule.WholeWord {
					result, _ = e.replaceWholeWords(result, rule.Original, rule.Replacement)
				} else {
					result = strings.ReplaceAll(result, rule.Original, rule.Replacement)
				}
			} else {
				if rule.WholeWord {
					result, _ = e.replaceWholeWordsInsensitive(result, rule.Original, rule.Replacement)
				} else {
					result = e.replaceAllInsensitive(result, rule.Original, rule.Replacement)
				}
			}
		}
	}
	return result
}

func TestReplacementEngine_DryRunChanges(t *testing.T) {
	rules := []ReplacementRule{
		{
			Original:      "hello",
			Replacement:   "hi",
			Regex:         false,
			CaseSensitive: true,
			WholeWord:     false,
		},
		{
			Original:      "world",
			Replacement:   "universe",
			Regex:         false,
			CaseSensitive: true,
			WholeWord:     false,
		},
	}

	engine, err := NewReplacementEngine(rules, []string{}, []string{})
	if err != nil {
		t.Fatalf("Failed to create replacement engine: %v", err)
	}

	content := "hello world\nhello there\nworld hello"
	
	// Test findChanges method
	changes := engine.findChanges(content, rules[0]) // Test "hello" rule
	if len(changes) != 3 { // Should find 3 occurrences of "hello"
		t.Errorf("Expected 3 changes for 'hello', got %d", len(changes))
	}

	// Verify first change
	if changes[0].LineNumber != 1 {
		t.Errorf("Expected first change on line 1, got line %d", changes[0].LineNumber)
	}
	if changes[0].Original != "hello" {
		t.Errorf("Expected original 'hello', got %q", changes[0].Original)
	}
	if changes[0].Replacement != "hi" {
		t.Errorf("Expected replacement 'hi', got %q", changes[0].Replacement)
	}

	// Test second rule
	changes2 := engine.findChanges(content, rules[1]) // Test "world" rule
	if len(changes2) != 2 { // Should find 2 occurrences of "world"
		t.Errorf("Expected 2 changes for 'world', got %d", len(changes2))
	}
}