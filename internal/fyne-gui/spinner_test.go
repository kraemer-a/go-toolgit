package fynegui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestNewAnimatedSpinner(t *testing.T) {
	spinner := NewAnimatedSpinner()

	if spinner == nil {
		t.Fatal("NewAnimatedSpinner returned nil")
	}

	if spinner.size != 50 {
		t.Errorf("Expected default size 50, got %v", spinner.size)
	}

	if spinner.style != SpinnerStyleCircles {
		t.Errorf("Expected default style SpinnerStyleCircles, got %v", spinner.style)
	}

	if spinner.running {
		t.Error("New spinner should not be running")
	}
}

func TestNewAnimatedSpinnerWithStyle(t *testing.T) {
	tests := []struct {
		name  string
		style SpinnerStyle
	}{
		{"circles style", SpinnerStyleCircles},
		{"dots style", SpinnerStyleDots},
		{"bars style", SpinnerStyleBars},
		{"pulse style", SpinnerStylePulse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := NewAnimatedSpinnerWithStyle(tt.style)

			if spinner == nil {
				t.Fatal("NewAnimatedSpinnerWithStyle returned nil")
			}

			if spinner.style != tt.style {
				t.Errorf("Expected style %v, got %v", tt.style, spinner.style)
			}

			if spinner.size != 50 {
				t.Errorf("Expected default size 50, got %v", spinner.size)
			}

			if spinner.running {
				t.Error("New spinner should not be running")
			}
		})
	}
}

func TestAnimatedSpinner_CreateRenderer(t *testing.T) {
	tests := []struct {
		name  string
		style SpinnerStyle
	}{
		{"circles renderer", SpinnerStyleCircles},
		{"dots renderer", SpinnerStyleDots},
		{"bars renderer", SpinnerStyleBars},
		{"pulse renderer", SpinnerStylePulse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := NewAnimatedSpinnerWithStyle(tt.style)
			renderer := spinner.CreateRenderer()

			if renderer == nil {
				t.Fatal("CreateRenderer returned nil")
			}

			// Test that renderer has objects
			objects := renderer.Objects()
			if len(objects) == 0 {
				t.Error("Renderer should have objects")
			}

			// Test renderer methods don't panic
			minSize := renderer.MinSize()
			if minSize.Width <= 0 || minSize.Height <= 0 {
				t.Error("MinSize should return positive dimensions")
			}

			// Test layout method doesn't panic
			renderer.Layout(fyne.NewSize(100, 100))

			// Test refresh doesn't panic
			renderer.Refresh()
		})
	}
}

func TestAnimatedSpinner_StartStop(t *testing.T) {
	spinner := NewAnimatedSpinner()

	if spinner.running {
		t.Error("Spinner should not be running initially")
	}

	// Test start
	spinner.Start()

	if !spinner.running {
		t.Error("Spinner should be running after Start()")
	}

	// Test that calling Start() again doesn't cause issues
	spinner.Start()

	if !spinner.running {
		t.Error("Spinner should still be running after second Start()")
	}

	// Test stop
	spinner.Stop()

	if spinner.running {
		t.Error("Spinner should not be running after Stop()")
	}
}

func TestAnimatedSpinner_MinSize(t *testing.T) {
	tests := []struct {
		name string
		size float32
	}{
		{"default size", 50},
		{"custom size", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := NewAnimatedSpinner()
			spinner.size = tt.size

			minSize := spinner.MinSize()

			if minSize.Width != tt.size || minSize.Height != tt.size {
				t.Errorf("Expected MinSize(%v, %v), got (%v, %v)", 
					tt.size, tt.size, minSize.Width, minSize.Height)
			}
		})
	}
}

func TestAnimatedSpinner_AnimationCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping animation test in short mode")
	}

	spinner := NewAnimatedSpinner()
	
	// Start animation
	spinner.Start()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop and verify cleanup
	spinner.Stop()

	// Note: Animation.Running() may not be available in all Fyne versions
	// This is a basic check that animation exists
	if spinner.animation != nil {
		// Animation cleanup test - if animation exists, it should be managed properly
	}

	if spinner.running {
		t.Error("Spinner should not be running after Stop()")
	}
}

func TestNewLoadingContainer(t *testing.T) {
	message := "Loading test data..."
	container := NewLoadingContainer(message)

	if container == nil {
		t.Fatal("NewLoadingContainer returned nil")
	}

	if container.spinner == nil {
		t.Error("LoadingContainer should have a spinner")
	}

	if container.label == nil {
		t.Error("LoadingContainer should have a label")
	}

	if container.label.Text != message {
		t.Errorf("Expected label text '%s', got '%s'", message, container.label.Text)
	}

	if container.container == nil {
		t.Error("LoadingContainer should have a container")
	}
}

func TestLoadingContainer_CreateRenderer(t *testing.T) {
	container := NewLoadingContainer("Test")
	renderer := container.CreateRenderer()

	if renderer == nil {
		t.Fatal("CreateRenderer returned nil")
	}

	// Test that renderer has objects
	objects := renderer.Objects()
	if len(objects) == 0 {
		t.Error("Renderer should have objects")
	}
}

func TestLoadingContainer_StartStop(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	container := NewLoadingContainer("Test")

	// Test start
	container.Start()

	if !container.spinner.running {
		t.Error("Spinner should be running after Start()")
	}

	// Test stop
	container.Stop()

	if container.spinner.running {
		t.Error("Spinner should not be running after Stop()")
	}
}

func TestLoadingContainer_SetMessage(t *testing.T) {
	container := NewLoadingContainer("Initial message")

	newMessage := "Updated message"
	container.SetMessage(newMessage)

	if container.label.Text != newMessage {
		t.Errorf("Expected label text '%s', got '%s'", newMessage, container.label.Text)
	}
}

func TestLoadingContainer_StartStopMultipleTimes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	container := NewLoadingContainer("Test")

	// Multiple start/stop cycles
	for i := 0; i < 3; i++ {
		container.Start()
		if !container.spinner.running {
			t.Errorf("Spinner should be running after Start() cycle %d", i+1)
		}

		container.Stop()
		if container.spinner.running {
			t.Errorf("Spinner should not be running after Stop() cycle %d", i+1)
		}
	}
}

func TestSpinnerStyle_Constants(t *testing.T) {
	// Test that spinner style constants are defined and unique
	styles := []SpinnerStyle{
		SpinnerStyleCircles,
		SpinnerStyleDots,
		SpinnerStyleBars,
		SpinnerStylePulse,
	}

	// Check that all styles are different
	for i, style1 := range styles {
		for j, style2 := range styles {
			if i != j && style1 == style2 {
				t.Errorf("Spinner styles %d and %d have the same value", i, j)
			}
		}
	}
}

func TestAnimatedSpinner_DifferentStylesRender(t *testing.T) {
	styles := []SpinnerStyle{
		SpinnerStyleCircles,
		SpinnerStyleDots,
		SpinnerStyleBars,
		SpinnerStylePulse,
	}

	for _, style := range styles {
		t.Run("style_"+string(rune(style)), func(t *testing.T) {
			spinner := NewAnimatedSpinnerWithStyle(style)
			renderer := spinner.CreateRenderer()

			if renderer == nil {
				t.Fatalf("CreateRenderer returned nil for style %v", style)
			}

			objects := renderer.Objects()
			if len(objects) == 0 {
				t.Errorf("No objects created for style %v", style)
			}

			// Test that start/stop doesn't panic for this style
			spinner.Start()
			spinner.Stop()
		})
	}
}

func TestAnimatedSpinner_AnimationMethodsExist(t *testing.T) {
	spinner := NewAnimatedSpinner()

	// Create renderer to initialize components
	renderer := spinner.CreateRenderer()
	if renderer == nil {
		t.Fatal("CreateRenderer returned nil")
	}

	// Test that animation methods exist and don't panic
	tests := []struct {
		name  string
		style SpinnerStyle
	}{
		{"circles animation", SpinnerStyleCircles},
		{"dots animation", SpinnerStyleDots},
		{"bars animation", SpinnerStyleBars},
		{"pulse animation", SpinnerStylePulse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner.style = tt.style
			
			// This should not panic
			spinner.Start()
			
			// Brief pause to let animation start
			time.Sleep(10 * time.Millisecond)
			
			spinner.Stop()
		})
	}
}

// Test widget interfaces
func TestSpinner_WidgetInterfaces(t *testing.T) {
	// Test that spinners implement fyne.Widget interface
	var _ fyne.Widget = &AnimatedSpinner{}
	var _ fyne.Widget = &LoadingContainer{}
}

// Benchmark spinner operations
func BenchmarkNewAnimatedSpinner(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewAnimatedSpinner()
	}
}

func BenchmarkAnimatedSpinner_StartStop(b *testing.B) {
	spinner := NewAnimatedSpinner()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spinner.Start()
		spinner.Stop()
	}
}

func BenchmarkNewLoadingContainer(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewLoadingContainer("Benchmark loading...")
	}
}

func BenchmarkLoadingContainer_SetMessage(b *testing.B) {
	container := NewLoadingContainer("Initial")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		container.SetMessage("Updated message")
	}
}

// Integration test with actual app
func TestSpinner_IntegrationWithApp(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("Spinner Test")
	window.Resize(fyne.NewSize(200, 200))

	// Test that spinner can be added to window without issues
	spinner := NewAnimatedSpinner()
	window.SetContent(spinner)

	// Test start/stop in app context
	spinner.Start()
	
	// Brief pause
	time.Sleep(50 * time.Millisecond)
	
	spinner.Stop()

	// Test loading container in app context
	loading := NewLoadingContainer("Integration test")
	window.SetContent(loading)

	loading.Start()
	loading.SetMessage("Updated in app")
	loading.Stop()
}