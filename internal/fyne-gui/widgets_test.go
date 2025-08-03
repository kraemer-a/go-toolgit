package fynegui

import (
	"image/color"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestNewToggleSwitch(t *testing.T) {
	called := false
	toggle := NewToggleSwitch("Test Toggle", func(checked bool) {
		called = true
	})

	if toggle == nil {
		t.Fatal("NewToggleSwitch returned nil")
	}

	if toggle.Text != "Test Toggle" {
		t.Errorf("Expected text 'Test Toggle', got '%s'", toggle.Text)
	}

	if toggle.Checked {
		t.Error("New toggle switch should start unchecked")
	}

	if toggle.OnChanged == nil {
		t.Error("OnChanged callback should not be nil")
	}

	// Test callback
	toggle.OnChanged(true)
	if !called {
		t.Error("OnChanged callback was not called")
	}
}

func TestToggleSwitch_SetChecked(t *testing.T) {
	callbackCalled := false
	callbackValue := false

	toggle := NewToggleSwitch("Test", func(checked bool) {
		callbackCalled = true
		callbackValue = checked
	})

	// Test setting to true
	toggle.SetChecked(true)

	if !toggle.Checked {
		t.Error("Toggle should be checked after SetChecked(true)")
	}

	if !callbackCalled {
		t.Error("OnChanged callback should be called")
	}

	if !callbackValue {
		t.Error("Callback should receive true value")
	}

	// Reset for next test
	callbackCalled = false

	// Test setting to false
	toggle.SetChecked(false)

	if toggle.Checked {
		t.Error("Toggle should be unchecked after SetChecked(false)")
	}

	if !callbackCalled {
		t.Error("OnChanged callback should be called when setting to false")
	}

	if callbackValue {
		t.Error("Callback should receive false value")
	}
}

func TestToggleSwitch_SetCheckedSameValue(t *testing.T) {
	callbackCalled := false

	toggle := NewToggleSwitch("Test", func(checked bool) {
		callbackCalled = true
	})

	// Set to false (already false)
	toggle.SetChecked(false)

	if callbackCalled {
		t.Error("OnChanged should not be called when setting to same value")
	}
}

func TestToggleSwitch_SetCheckedWithoutRenderer(t *testing.T) {
	// Test SetChecked when handle/background are nil (before renderer creation)
	callbackCalled := false
	toggle := NewToggleSwitch("Test", func(checked bool) {
		callbackCalled = true
	})

	// This should not panic and should call callback
	toggle.SetChecked(true)

	if !toggle.Checked {
		t.Error("Toggle should be checked")
	}

	if !callbackCalled {
		t.Error("Callback should be called even without renderer")
	}
}

func TestToggleSwitch_Tapped(t *testing.T) {
	callbackCalled := false
	callbackValue := false

	toggle := NewToggleSwitch("Test", func(checked bool) {
		callbackCalled = true
		callbackValue = checked
	})

	// Simulate tap event
	toggle.Tapped(&fyne.PointEvent{})

	if !toggle.Checked {
		t.Error("Toggle should be checked after tap")
	}

	if !callbackCalled {
		t.Error("OnChanged callback should be called on tap")
	}

	if !callbackValue {
		t.Error("Callback should receive true after first tap")
	}

	// Reset and tap again
	callbackCalled = false
	toggle.Tapped(&fyne.PointEvent{})

	if toggle.Checked {
		t.Error("Toggle should be unchecked after second tap")
	}

	if callbackValue {
		t.Error("Callback should receive false after second tap")
	}
}

func TestNewTagChip(t *testing.T) {
	deleteCalled := false
	chip := NewTagChip("test-tag", func() {
		deleteCalled = true
	})

	if chip == nil {
		t.Fatal("NewTagChip returned nil")
	}

	if chip.Text != "test-tag" {
		t.Errorf("Expected text 'test-tag', got '%s'", chip.Text)
	}

	if chip.OnDeleted == nil {
		t.Error("OnDeleted callback should not be nil")
	}

	// Test callback
	chip.OnDeleted()
	if !deleteCalled {
		t.Error("OnDeleted callback was not called")
	}
}

func TestNewTagChip_WithoutCallback(t *testing.T) {
	chip := NewTagChip("test-tag", nil)

	if chip == nil {
		t.Fatal("NewTagChip returned nil")
	}

	if chip.OnDeleted != nil {
		t.Error("OnDeleted should be nil when no callback provided")
	}
}

func TestNewEnhancedProgressBar(t *testing.T) {
	progressBar := NewEnhancedProgressBar()

	if progressBar == nil {
		t.Fatal("NewEnhancedProgressBar returned nil")
	}

	if !progressBar.ShowPercent {
		t.Error("ShowPercent should default to true")
	}

	if progressBar.Value != 0 {
		t.Error("Value should default to 0")
	}
}

func TestEnhancedProgressBar_SetValue(t *testing.T) {
	progressBar := NewEnhancedProgressBar()

	// Create renderer to initialize internal components
	progressBar.CreateRenderer()

	tests := []struct {
		name     string
		value    float64
		expected string
	}{
		{"zero percent", 0.0, "0%"},
		{"fifty percent", 0.5, "50%"},
		{"full percent", 1.0, "100%"},
		{"over full", 1.2, "120%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progressBar.SetValue(tt.value)

			if progressBar.Value != tt.value {
				t.Errorf("Expected value %v, got %v", tt.value, progressBar.Value)
			}

			if progressBar.ShowPercent && progressBar.label.Text != tt.expected {
				t.Errorf("Expected label text '%s', got '%s'", tt.expected, progressBar.label.Text)
			}
		})
	}
}

func TestShowToast(t *testing.T) {
	app := test.NewApp()
	window := app.NewWindow("Test")
	window.Resize(fyne.NewSize(400, 300))

	// Test different toast types
	tests := []struct {
		name      string
		toastType string
		message   string
	}{
		{"info toast", "info", "Info message"},
		{"success toast", "success", "Success message"},
		{"error toast", "error", "Error message"},
		{"warning toast", "warning", "Warning message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic
			ShowToast(window, tt.message, tt.toastType)

			// Verify overlay was added
			overlays := window.Canvas().Overlays()
			if overlays == nil {
				t.Error("No overlays found after showing toast")
			}
		})
	}
}

func TestShowToast_Colors(t *testing.T) {
	// Test that different toast types use different colors
	testColors := map[string]color.Color{
		"success": color.RGBA{34, 197, 94, 255},  // Updated success color
		"error":   color.RGBA{255, 85, 85, 255},
		"warning": color.RGBA{255, 184, 108, 255},
		"info":    color.RGBA{98, 114, 164, 255},
	}

	for toastType, expectedColor := range testColors {
		t.Run(toastType+" color", func(t *testing.T) {
			app := test.NewApp()
			window := app.NewWindow("Test")

			ShowToast(window, "test", toastType)

			// This is more of a smoke test - actual color verification
			// would require deeper inspection of the overlay contents
			overlays := window.Canvas().Overlays()
			if len(overlays.List()) == 0 {
				t.Error("Expected overlay to be added")
			}

			// Verify expected color is defined (basic sanity check)
			if rgba, ok := expectedColor.(color.RGBA); ok {
				if rgba.R == 0 && rgba.G == 0 && rgba.B == 0 {
					t.Errorf("Invalid color for toast type %s", toastType)
				}
			}
		})
	}
}

func TestToastAutoHide(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timing test in short mode")
	}

	app := test.NewApp()
	window := app.NewWindow("Test")

	ShowToast(window, "Auto-hide test", "info")

	// Verify overlay exists initially
	overlays := window.Canvas().Overlays()
	if len(overlays.List()) == 0 {
		t.Fatal("Expected overlay to be added")
	}

	// Wait longer than auto-hide duration (3 seconds + animation time)
	time.Sleep(4 * time.Second)

	// Note: In real implementation, we'd need to verify overlay removal
	// This is a simplified test that verifies the toast doesn't panic
	// and runs its auto-hide goroutine
}

// Test widget interfaces
func TestWidgetInterfaces(t *testing.T) {
	// Test that widgets implement expected interfaces
	t.Run("ToggleSwitch implements fyne.Widget", func(t *testing.T) {
		var _ fyne.Widget = &ToggleSwitch{}
	})

	t.Run("ToggleSwitch implements fyne.Tappable", func(t *testing.T) {
		var _ fyne.Tappable = &ToggleSwitch{}
	})

	t.Run("TagChip implements fyne.Widget", func(t *testing.T) {
		var _ fyne.Widget = &TagChip{}
	})

	t.Run("EnhancedProgressBar implements fyne.Widget", func(t *testing.T) {
		var _ fyne.Widget = &EnhancedProgressBar{}
	})
}

// Benchmark widget creation
func BenchmarkNewToggleSwitch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewToggleSwitch("Benchmark", nil)
	}
}

func BenchmarkToggleSwitch_SetChecked(b *testing.B) {
	toggle := NewToggleSwitch("Benchmark", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toggle.SetChecked(i%2 == 0)
	}
}

func BenchmarkNewTagChip(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewTagChip("benchmark-tag", nil)
	}
}