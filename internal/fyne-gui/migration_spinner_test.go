package fynegui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestMigrationSpinnersAppearSequentially verifies that only one spinner is active at a time
func TestMigrationSpinnersAppearSequentially(t *testing.T) {
	// Create test app
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("Test Migration Spinners")
	defer window.Close()

	// Create FyneApp instance with progress container
	fyneApp := &FyneApp{
		app:               app,
		window:            window,
		progressContainer: container.New(layout.NewVBoxLayout()),
	}

	// Define test migration steps
	steps := []MigrationStep{
		{Description: "Validating source repository", Status: "pending", Progress: 0},
		{Description: "Creating target repository", Status: "pending", Progress: 0},
		{Description: "Cloning source repository", Status: "pending", Progress: 0},
		{Description: "Pushing to target repository", Status: "pending", Progress: 0},
		{Description: "Configuring teams", Status: "pending", Progress: 0},
		{Description: "Setting up webhooks", Status: "pending", Progress: 0},
		{Description: "Triggering pipeline", Status: "pending", Progress: 0},
	}

	// Test 1: Initial state - all pending, no spinners
	fyneApp.displayMigrationSteps(steps)
	if countActiveSpinners(fyneApp.progressContainer) != 0 {
		t.Error("Expected no spinners when all steps are pending")
	}

	// Test 2: First step running - exactly one spinner
	steps[0].Status = "running"
	steps[0].Message = "Connecting to Bitbucket..."
	fyneApp.displayMigrationSteps(steps)

	activeSpinners := countActiveSpinners(fyneApp.progressContainer)
	if activeSpinners != 1 {
		t.Errorf("Expected 1 spinner for first running step, got %d", activeSpinners)
	}

	// Test 3: Transition - first completed, second running
	steps[0].Status = "completed"
	steps[0].Message = "Source repository validated"
	steps[1].Status = "running"
	steps[1].Message = "Creating GitHub repository..."
	fyneApp.displayMigrationSteps(steps)

	activeSpinners = countActiveSpinners(fyneApp.progressContainer)
	if activeSpinners != 1 {
		t.Errorf("Expected 1 spinner after transition, got %d", activeSpinners)
	}

	// Test 4: Multiple transitions - verify sequential spinner movement
	for i := 1; i < len(steps); i++ {
		// Complete current step
		steps[i].Status = "completed"
		steps[i].Progress = 100
		steps[i].Message = "Step completed"

		// Start next step if not the last
		if i+1 < len(steps) {
			steps[i+1].Status = "running"
			steps[i+1].Message = "Processing..."
		}

		fyneApp.displayMigrationSteps(steps)

		// Verify only one spinner active (or none if last step)
		expectedSpinners := 0
		if i+1 < len(steps) {
			expectedSpinners = 1
		}

		activeSpinners = countActiveSpinners(fyneApp.progressContainer)
		if activeSpinners != expectedSpinners {
			t.Errorf("Step %d: Expected %d spinner(s), got %d", i, expectedSpinners, activeSpinners)
		}
	}

	// Test 5: All completed - no spinners
	for i := range steps {
		steps[i].Status = "completed"
		steps[i].Progress = 100
	}
	fyneApp.displayMigrationSteps(steps)

	if countActiveSpinners(fyneApp.progressContainer) != 0 {
		t.Error("Expected no spinners when all steps are completed")
	}
}

// TestDisplayMigrationStepsSpinnerBehavior tests spinner creation for different statuses
func TestDisplayMigrationStepsSpinnerBehavior(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("Test Spinner Behavior")
	defer window.Close()

	fyneApp := &FyneApp{
		app:               app,
		window:            window,
		progressContainer: container.New(layout.NewVBoxLayout()),
	}

	testCases := []struct {
		name          string
		status        string
		expectSpinner bool
		expectedIcon  string
	}{
		{"Pending step", "pending", false, "⏳"},
		{"Running step", "running", true, "🔄"},
		{"Completed step", "completed", false, "✅"},
		{"Failed step", "failed", false, "❌"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			steps := []MigrationStep{
				{Description: "Test step", Status: tc.status, Progress: 0},
			}

			fyneApp.displayMigrationSteps(steps)

			hasSpinner := countActiveSpinners(fyneApp.progressContainer) > 0
			if hasSpinner != tc.expectSpinner {
				t.Errorf("Status '%s': expected spinner=%v, got %v", tc.status, tc.expectSpinner, hasSpinner)
			}

			// Verify the status icon is present in the label
			if !verifyStatusIcon(fyneApp.progressContainer, tc.expectedIcon) {
				t.Errorf("Status '%s': expected icon '%s' not found", tc.status, tc.expectedIcon)
			}
		})
	}
}

// TestMigrationProgressUIComponents tests the UI component structure
func TestMigrationProgressUIComponents(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("Test UI Components")
	defer window.Close()

	fyneApp := &FyneApp{
		app:               app,
		window:            window,
		progressContainer: container.New(layout.NewVBoxLayout()),
	}

	// Create a running step to test spinner properties
	steps := []MigrationStep{
		{Description: "Running step", Status: "running", Progress: 50, Message: "In progress..."},
	}

	fyneApp.displayMigrationSteps(steps)

	// Test 1: Verify container structure
	if len(fyneApp.progressContainer.Objects) != 1 {
		t.Errorf("Expected 1 object in progress container, got %d", len(fyneApp.progressContainer.Objects))
	}

	// Test 2: For running step, verify it's a container with HBox layout
	if stepContainer, ok := fyneApp.progressContainer.Objects[0].(*fyne.Container); ok {
		// Check for HBox layout (should have label and spinner)
		if len(stepContainer.Objects) < 2 {
			t.Error("Running step container should have at least 2 objects (label and spinner)")
		}

		// Verify first object is a label
		if _, ok := stepContainer.Objects[0].(*widget.Label); !ok {
			t.Error("First object should be a label")
		}

		// Find the spinner (might be after a spacer)
		foundSpinner := false
		for _, obj := range stepContainer.Objects {
			if _, ok := obj.(*AnimatedSpinner); ok {
				foundSpinner = true
				break
			}
		}

		if !foundSpinner {
			t.Error("Running step should contain an AnimatedSpinner")
		}
	} else {
		t.Error("Running step should be in a container")
	}
}

// TestMigrationStepTransitionTiming simulates realistic step transitions with delays
func TestMigrationStepTransitionTiming(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	window := app.NewWindow("Test Transition Timing")
	defer window.Close()

	fyneApp := &FyneApp{
		app:               app,
		window:            window,
		progressContainer: container.New(layout.NewVBoxLayout()),
	}

	steps := []MigrationStep{
		{Description: "Step 1", Status: "pending", Progress: 0},
		{Description: "Step 2", Status: "pending", Progress: 0},
		{Description: "Step 3", Status: "pending", Progress: 0},
	}

	// Simulate migration with delays
	transitions := []struct {
		stepIndex int
		newStatus string
		delay     time.Duration
	}{
		{0, "running", 100 * time.Millisecond},
		{0, "completed", 200 * time.Millisecond},
		{1, "running", 100 * time.Millisecond},
		{1, "completed", 200 * time.Millisecond},
		{2, "running", 100 * time.Millisecond},
		{2, "completed", 200 * time.Millisecond},
	}

	for _, transition := range transitions {
		time.Sleep(transition.delay)

		steps[transition.stepIndex].Status = transition.newStatus
		if transition.newStatus == "completed" {
			steps[transition.stepIndex].Progress = 100
		}

		fyneApp.displayMigrationSteps(steps)

		// Count active spinners after each transition
		activeSpinners := countActiveSpinners(fyneApp.progressContainer)

		// Should have 1 spinner if any step is running, 0 otherwise
		expectedSpinners := 0
		for _, step := range steps {
			if step.Status == "running" {
				expectedSpinners = 1
				break
			}
		}

		if activeSpinners != expectedSpinners {
			t.Errorf("After transition (step %d to %s): expected %d spinner(s), got %d",
				transition.stepIndex, transition.newStatus, expectedSpinners, activeSpinners)
		}
	}
}

// Helper function to count active spinners in the container
func countActiveSpinners(container *fyne.Container) int {
	count := 0
	for _, obj := range container.Objects {
		count += countSpinnersInObject(obj)
	}
	return count
}

// Recursive helper to find spinners in nested containers
func countSpinnersInObject(obj fyne.CanvasObject) int {
	if _, ok := obj.(*AnimatedSpinner); ok {
		return 1
	}

	if container, ok := obj.(*fyne.Container); ok {
		count := 0
		for _, child := range container.Objects {
			count += countSpinnersInObject(child)
		}
		return count
	}

	return 0
}

// Helper function to verify status icon in labels
func verifyStatusIcon(container *fyne.Container, expectedIcon string) bool {
	for _, obj := range container.Objects {
		if found := checkObjectForIcon(obj, expectedIcon); found {
			return true
		}
	}
	return false
}

// Recursive helper to check for icon in object text
func checkObjectForIcon(obj fyne.CanvasObject, icon string) bool {
	if label, ok := obj.(*widget.Label); ok {
		if containsIcon(label.Text, icon) {
			return true
		}
	}

	if container, ok := obj.(*fyne.Container); ok {
		for _, child := range container.Objects {
			if checkObjectForIcon(child, icon) {
				return true
			}
		}
	}

	return false
}

// Helper to check if text contains the icon
func containsIcon(text, icon string) bool {
	// Simple contains check - icons are at the beginning of the text
	return len(text) >= len(icon) && text[:len(icon)] == icon
}
