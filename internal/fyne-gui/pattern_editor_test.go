package fynegui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestNewPatternEditor(t *testing.T) {
	patterns := []string{"*.go", "*.js"}
	callbackCalled := false
	var callbackPatterns []string

	editor := NewPatternEditor("Enter pattern", patterns, func(p []string) {
		callbackCalled = true
		callbackPatterns = p
	})

	if editor == nil {
		t.Fatal("NewPatternEditor returned nil")
	}

	if len(editor.patterns) != 2 {
		t.Errorf("Expected 2 patterns, got %d", len(editor.patterns))
	}

	if editor.patterns[0] != "*.go" || editor.patterns[1] != "*.js" {
		t.Errorf("Patterns not set correctly: %v", editor.patterns)
	}

	if editor.onChanged == nil {
		t.Error("onChanged callback should not be nil")
	}

	// Test callback
	editor.onChanged([]string{"*.test"})
	if !callbackCalled {
		t.Error("Callback was not called")
	}
	if len(callbackPatterns) != 1 || callbackPatterns[0] != "*.test" {
		t.Errorf("Callback received wrong patterns: %v", callbackPatterns)
	}
}

func TestPatternEditor_AddPattern(t *testing.T) {
	callbackCalled := false
	var callbackPatterns []string

	editor := NewPatternEditor("Test", []string{}, func(p []string) {
		callbackCalled = true
		callbackPatterns = p
	})

	// Test adding valid pattern
	editor.addPattern("*.go")

	if len(editor.patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(editor.patterns))
	}

	if editor.patterns[0] != "*.go" {
		t.Errorf("Expected pattern '*.go', got '%s'", editor.patterns[0])
	}

	if !callbackCalled {
		t.Error("Callback should be called when pattern is added")
	}

	if len(callbackPatterns) != 1 || callbackPatterns[0] != "*.go" {
		t.Errorf("Callback received wrong patterns: %v", callbackPatterns)
	}
}

func TestPatternEditor_AddPatternEmpty(t *testing.T) {
	editor := NewPatternEditor("Test", []string{}, func(p []string) {})

	// Test adding empty pattern
	editor.addPattern("")

	if len(editor.patterns) != 0 {
		t.Error("Empty pattern should not be added")
	}

	// Test adding whitespace-only pattern
	editor.addPattern("   ")

	if len(editor.patterns) != 0 {
		t.Error("Whitespace-only pattern should not be added")
	}
}

func TestPatternEditor_AddPatternDuplicate(t *testing.T) {
	// Create a test app and window for toast functionality
	app := test.NewApp()
	window := app.NewWindow("Test")

	editor := NewPatternEditor("Test", []string{"*.go"}, func(p []string) {})

	// Test adding duplicate pattern
	editor.addPattern("*.go")

	if len(editor.patterns) != 1 {
		t.Error("Duplicate pattern should not be added")
	}

	// Verify original pattern is still there
	if editor.patterns[0] != "*.go" {
		t.Error("Original pattern should remain unchanged")
	}

	window.Close()
	app.Quit()
}

func TestPatternEditor_RemovePattern(t *testing.T) {
	callbackCalled := false
	var callbackPatterns []string

	editor := NewPatternEditor("Test", []string{"*.go", "*.js", "*.py"}, func(p []string) {
		callbackCalled = true
		callbackPatterns = p
	})

	// Remove middle pattern
	editor.removePattern("*.js")

	if len(editor.patterns) != 2 {
		t.Errorf("Expected 2 patterns after removal, got %d", len(editor.patterns))
	}

	expectedPatterns := []string{"*.go", "*.py"}
	for i, expected := range expectedPatterns {
		if i >= len(editor.patterns) || editor.patterns[i] != expected {
			t.Errorf("Expected patterns %v, got %v", expectedPatterns, editor.patterns)
			break
		}
	}

	if !callbackCalled {
		t.Error("Callback should be called when pattern is removed")
	}

	if len(callbackPatterns) != 2 {
		t.Errorf("Callback should receive 2 patterns, got %d", len(callbackPatterns))
	}
}

func TestPatternEditor_RemovePatternNotFound(t *testing.T) {
	callbackCalled := false

	editor := NewPatternEditor("Test", []string{"*.go"}, func(p []string) {
		callbackCalled = true
	})

	// Try to remove non-existent pattern
	editor.removePattern("*.nonexistent")

	if len(editor.patterns) != 1 {
		t.Error("Pattern count should remain unchanged when removing non-existent pattern")
	}

	if editor.patterns[0] != "*.go" {
		t.Error("Existing pattern should remain unchanged")
	}

	if !callbackCalled {
		t.Error("Callback should still be called even when pattern not found")
	}
}

func TestPatternEditor_RemoveAllPatterns(t *testing.T) {
	callbackCalled := false
	var callbackPatterns []string

	editor := NewPatternEditor("Test", []string{"*.go", "*.js", "*.py"}, func(p []string) {
		callbackCalled = true
		callbackPatterns = p
	})

	// Remove all patterns
	editor.removeAllPatterns()

	if len(editor.patterns) != 0 {
		t.Errorf("Expected 0 patterns after removeAll, got %d", len(editor.patterns))
	}

	if !callbackCalled {
		t.Error("Callback should be called when all patterns are removed")
	}

	if len(callbackPatterns) != 0 {
		t.Errorf("Callback should receive empty slice, got %v", callbackPatterns)
	}
}

func TestPatternEditor_RemoveAllPatternsEmpty(t *testing.T) {
	callbackCalled := false

	editor := NewPatternEditor("Test", []string{}, func(p []string) {
		callbackCalled = true
	})

	// Try to remove all when already empty
	editor.removeAllPatterns()

	if len(editor.patterns) != 0 {
		t.Error("Patterns should remain empty")
	}

	if callbackCalled {
		t.Error("Callback should not be called when removing all from empty list")
	}
}

func TestPatternEditor_SetPatterns(t *testing.T) {
	editor := NewPatternEditor("Test", []string{"*.go"}, func(p []string) {})

	newPatterns := []string{"*.js", "*.ts", "*.jsx"}
	editor.SetPatterns(newPatterns)

	if len(editor.patterns) != 3 {
		t.Errorf("Expected 3 patterns, got %d", len(editor.patterns))
	}

	for i, expected := range newPatterns {
		if i >= len(editor.patterns) || editor.patterns[i] != expected {
			t.Errorf("Expected patterns %v, got %v", newPatterns, editor.patterns)
			break
		}
	}
}

func TestPatternEditor_GetPatterns(t *testing.T) {
	patterns := []string{"*.go", "*.js"}
	editor := NewPatternEditor("Test", patterns, func(p []string) {})

	result := editor.GetPatterns()

	if len(result) != len(patterns) {
		t.Errorf("Expected %d patterns, got %d", len(patterns), len(result))
	}

	for i, expected := range patterns {
		if i >= len(result) || result[i] != expected {
			t.Errorf("Expected patterns %v, got %v", patterns, result)
			break
		}
	}
}

func TestPatternEditor_GetPatternsAsString(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		expected string
	}{
		{"empty patterns", []string{}, ""},
		{"single pattern", []string{"*.go"}, "*.go"},
		{"multiple patterns", []string{"*.go", "*.js", "*.py"}, "*.go,*.js,*.py"},
		{"patterns with spaces", []string{"*.go", "src/*.js"}, "*.go,src/*.js"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editor := NewPatternEditor("Test", tt.patterns, func(p []string) {})
			result := editor.GetPatternsAsString()

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestPatternEditor_EntrySubmitted(t *testing.T) {
	callbackCalled := false
	var callbackPatterns []string

	editor := NewPatternEditor("Test", []string{}, func(p []string) {
		callbackCalled = true
		callbackPatterns = p
	})

	// Simulate entry submission
	if editor.entry.OnSubmitted != nil {
		editor.entry.SetText("*.test")
		editor.entry.OnSubmitted("*.test")

		if len(editor.patterns) != 1 {
			t.Errorf("Expected 1 pattern after entry submission, got %d", len(editor.patterns))
		}

		if editor.patterns[0] != "*.test" {
			t.Errorf("Expected pattern '*.test', got '%s'", editor.patterns[0])
		}

		if !callbackCalled {
			t.Error("Callback should be called after entry submission")
		}

		if len(callbackPatterns) != 1 || callbackPatterns[0] != "*.test" {
			t.Errorf("Callback received wrong patterns: %v", callbackPatterns)
		}

		// Entry should be cleared after submission
		if editor.entry.Text != "" {
			t.Error("Entry text should be cleared after submission")
		}
	} else {
		t.Error("Entry OnSubmitted callback should not be nil")
	}
}

func TestPatternEditor_AddButtonClick(t *testing.T) {
	callbackCalled := false

	editor := NewPatternEditor("Test", []string{}, func(p []string) {
		callbackCalled = true
	})

	// Simulate add button click
	editor.entry.SetText("*.button")
	if editor.addButton.OnTapped != nil {
		editor.addButton.OnTapped()

		if len(editor.patterns) != 1 {
			t.Errorf("Expected 1 pattern after button click, got %d", len(editor.patterns))
		}

		if editor.patterns[0] != "*.button" {
			t.Errorf("Expected pattern '*.button', got '%s'", editor.patterns[0])
		}

		if !callbackCalled {
			t.Error("Callback should be called after button click")
		}

		// Entry should be cleared after adding
		if editor.entry.Text != "" {
			t.Error("Entry text should be cleared after adding pattern")
		}
	} else {
		t.Error("Add button OnTapped callback should not be nil")
	}
}

func TestPatternEditor_RemoveAllButtonClick(t *testing.T) {
	callbackCalled := false
	var callbackPatterns []string

	editor := NewPatternEditor("Test", []string{"*.go", "*.js"}, func(p []string) {
		callbackCalled = true
		callbackPatterns = p
	})

	// Simulate remove all button click
	if editor.removeAllButton.OnTapped != nil {
		editor.removeAllButton.OnTapped()

		if len(editor.patterns) != 0 {
			t.Errorf("Expected 0 patterns after remove all button click, got %d", len(editor.patterns))
		}

		if !callbackCalled {
			t.Error("Callback should be called after remove all button click")
		}

		if len(callbackPatterns) != 0 {
			t.Errorf("Callback should receive empty slice, got %v", callbackPatterns)
		}
	} else {
		t.Error("Remove all button OnTapped callback should not be nil")
	}
}

// Test widget interface compliance
func TestPatternEditor_WidgetInterface(t *testing.T) {
	var _ fyne.Widget = &PatternEditor{}
}

// Benchmark pattern operations
func BenchmarkPatternEditor_AddPattern(b *testing.B) {
	editor := NewPatternEditor("Test", []string{}, func(p []string) {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		editor.addPattern("*.test")
		editor.patterns = []string{} // Reset for next iteration
	}
}

func BenchmarkPatternEditor_RemovePattern(b *testing.B) {
	// Setup with many patterns
	patterns := make([]string, 1000)
	for i := range patterns {
		patterns[i] = "*.test"
	}
	editor := NewPatternEditor("Test", patterns, func(p []string) {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(editor.patterns) > 0 {
			editor.removePattern(editor.patterns[0])
		}
	}
}

func BenchmarkPatternEditor_GetPatternsAsString(b *testing.B) {
	patterns := []string{"*.go", "*.js", "*.py", "*.ts", "*.jsx"}
	editor := NewPatternEditor("Test", patterns, func(p []string) {})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		editor.GetPatternsAsString()
	}
}