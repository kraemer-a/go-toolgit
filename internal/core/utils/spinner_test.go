package utils

import (
	"testing"
	"time"
)

func TestSpinner_Creation(t *testing.T) {
	spinner, err := NewSpinner("Test message")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	if spinner == nil {
		t.Error("Spinner should not be nil")
	}
}

func TestSpinner_InvalidMessage(t *testing.T) {
	spinner, err := NewSpinner("")
	if err != nil {
		t.Fatalf("Empty message should be allowed: %v", err)
	}

	if spinner == nil {
		t.Error("Spinner should not be nil even with empty message")
	}
}

func TestSpinner_StartStop(t *testing.T) {
	spinner, err := NewSpinner("Test spinner")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	// Test start
	err = spinner.Start()
	if err != nil {
		t.Fatalf("Failed to start spinner: %v", err)
	}

	// Give it a moment to spin
	time.Sleep(100 * time.Millisecond)

	// Test stop with success
	err = spinner.StopWithSuccess("Operation completed")
	if err != nil {
		t.Errorf("Failed to stop spinner with success: %v", err)
	}
}

func TestSpinner_StopWithFailure(t *testing.T) {
	spinner, err := NewSpinner("Test spinner")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	err = spinner.Start()
	if err != nil {
		t.Fatalf("Failed to start spinner: %v", err)
	}

	// Give it a moment to spin
	time.Sleep(100 * time.Millisecond)

	// Test stop with failure
	err = spinner.StopWithFailure("Operation failed")
	if err != nil {
		t.Errorf("Failed to stop spinner with failure: %v", err)
	}
}

func TestSpinner_UpdateMessage(t *testing.T) {
	spinner, err := NewSpinner("Initial message")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	err = spinner.Start()
	if err != nil {
		t.Fatalf("Failed to start spinner: %v", err)
	}

	// Update message
	spinner.UpdateMessage("Updated message")

	// Give it a moment to show the updated message
	time.Sleep(100 * time.Millisecond)

	err = spinner.StopWithSuccess("Done")
	if err != nil {
		t.Errorf("Failed to stop spinner: %v", err)
	}
}

func TestSpinner_MultipleOperations(t *testing.T) {
	messages := []string{
		"Step 1: Initializing",
		"Step 2: Processing",
		"Step 3: Finalizing",
	}

	for i, message := range messages {
		spinner, err := NewSpinner(message)
		if err != nil {
			t.Fatalf("Failed to create spinner for step %d: %v", i+1, err)
		}

		err = spinner.Start()
		if err != nil {
			t.Fatalf("Failed to start spinner for step %d: %v", i+1, err)
		}

		// Simulate work
		time.Sleep(50 * time.Millisecond)

		err = spinner.StopWithSuccess("Completed")
		if err != nil {
			t.Errorf("Failed to stop spinner for step %d: %v", i+1, err)
		}
	}
}

func TestSpinner_StopWithoutStart(t *testing.T) {
	spinner, err := NewSpinner("Test spinner")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	// Try to stop without starting
	err = spinner.StopWithSuccess("Should handle gracefully")
	// This should not panic or fail - the spinner implementation should handle this gracefully
}

func TestSpinner_DoubleStart(t *testing.T) {
	spinner, err := NewSpinner("Test spinner")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	err = spinner.Start()
	if err != nil {
		t.Fatalf("Failed to start spinner first time: %v", err)
	}

	// Try to start again
	err = spinner.Start()
	// This should either succeed (idempotent) or return an error - both are acceptable
	// The important thing is it shouldn't panic

	spinner.StopWithSuccess("Done")
}

func TestSpinner_LongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	spinner, err := NewSpinner("Long running operation")
	if err != nil {
		t.Fatalf("Failed to create spinner: %v", err)
	}

	err = spinner.Start()
	if err != nil {
		t.Fatalf("Failed to start spinner: %v", err)
	}

	// Update message several times to test dynamic updates
	messages := []string{
		"Processing step 1...",
		"Processing step 2...",
		"Processing step 3...",
		"Finalizing...",
	}

	for _, msg := range messages {
		spinner.UpdateMessage(msg)
		time.Sleep(200 * time.Millisecond)
	}

	err = spinner.StopWithSuccess("All steps completed successfully")
	if err != nil {
		t.Errorf("Failed to stop long-running spinner: %v", err)
	}
}

// Benchmark tests
func BenchmarkSpinner_CreateAndDestroy(b *testing.B) {
	for i := 0; i < b.N; i++ {
		spinner, err := NewSpinner("Benchmark test")
		if err != nil {
			b.Fatalf("Failed to create spinner: %v", err)
		}
		
		spinner.Start()
		spinner.StopWithSuccess("Done")
	}
}

func BenchmarkSpinner_UpdateMessage(b *testing.B) {
	spinner, err := NewSpinner("Initial message")
	if err != nil {
		b.Fatalf("Failed to create spinner: %v", err)
	}

	spinner.Start()
	defer spinner.StopWithSuccess("Done")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		spinner.UpdateMessage("Message update")
	}
}