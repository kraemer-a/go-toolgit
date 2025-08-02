package utils

import (
	"errors"
	"testing"
)

func TestNewValidationError(t *testing.T) {
	message := "validation failed"
	cause := errors.New("field is required")

	err := NewValidationError(message, cause)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "validation failed: field is required"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	// Test that it unwraps correctly
	if !errors.Is(err, cause) {
		t.Error("Error should wrap the original cause")
	}
}

func TestNewAuthError(t *testing.T) {
	message := "authentication failed"
	cause := errors.New("invalid token")

	err := NewAuthError(message, cause)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "authentication failed: invalid token"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	if !errors.Is(err, cause) {
		t.Error("Error should wrap the original cause")
	}
}

func TestNewNetworkError(t *testing.T) {
	message := "network request failed"
	cause := errors.New("connection timeout")

	err := NewNetworkError(message, cause)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "network request failed: connection timeout"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	if !errors.Is(err, cause) {
		t.Error("Error should wrap the original cause")
	}
}

func TestNewGitError(t *testing.T) {
	message := "git operation failed"
	cause := errors.New("repository not found")

	err := NewGitError(message, cause)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "git operation failed: repository not found"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	if !errors.Is(err, cause) {
		t.Error("Error should wrap the original cause")
	}
}

func TestNewProcessingError(t *testing.T) {
	message := "file processing failed"
	cause := errors.New("invalid content")

	err := NewProcessingError(message, cause)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	expectedMsg := "file processing failed: invalid content"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message %q, got %q", expectedMsg, err.Error())
	}

	if !errors.Is(err, cause) {
		t.Error("Error should wrap the original cause")
	}
}

func TestErrorWithoutCause(t *testing.T) {
	message := "simple error"

	err := NewValidationError(message, nil)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != message {
		t.Errorf("Expected error message %q, got %q", message, err.Error())
	}
}

func TestErrorChaining(t *testing.T) {
	// Create a chain of errors
	rootCause := errors.New("root cause")
	authErr := NewAuthError("auth failed", rootCause)
	networkErr := NewNetworkError("network failed", authErr)

	// Test that we can unwrap through the chain
	if !errors.Is(networkErr, authErr) {
		t.Error("networkErr should wrap authErr")
	}

	if !errors.Is(networkErr, rootCause) {
		t.Error("networkErr should ultimately wrap rootCause")
	}

	// Test error messages
	expectedMsg := "network failed: auth failed: root cause"
	if networkErr.Error() != expectedMsg {
		t.Errorf("Expected chained error message %q, got %q", expectedMsg, networkErr.Error())
	}
}

func TestErrorTypes(t *testing.T) {
	cause := errors.New("base error")

	tests := []struct {
		name        string
		constructor func(string, error) *GitHubReplaceError
		message     string
	}{
		{"ValidationError", NewValidationError, "validation error"},
		{"AuthError", NewAuthError, "auth error"},
		{"NetworkError", NewNetworkError, "network error"},
		{"GitError", NewGitError, "git error"},
		{"ProcessingError", NewProcessingError, "processing error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor(tt.message, cause)
			
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			expectedMsg := tt.message + ": base error"
			if err.Error() != expectedMsg {
				t.Errorf("Expected %q, got %q", expectedMsg, err.Error())
			}

			if !errors.Is(err, cause) {
				t.Error("Error should wrap the cause")
			}
		})
	}
}

func TestErrorWithEmptyMessage(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewValidationError("", cause)

	expectedMsg := ": underlying error"
	if err.Error() != expectedMsg {
		t.Errorf("Expected %q, got %q", expectedMsg, err.Error())
	}
}

func TestErrorWithNilCauseAndEmptyMessage(t *testing.T) {
	err := NewValidationError("", nil)

	if err.Error() != "" {
		t.Errorf("Expected empty string, got %q", err.Error())
	}
}

// Benchmark tests
func BenchmarkNewValidationError(b *testing.B) {
	cause := errors.New("test error")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewValidationError("validation failed", cause)
	}
}

func BenchmarkErrorChaining(b *testing.B) {
	rootCause := errors.New("root")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err1 := NewValidationError("validation", rootCause)
		err2 := NewAuthError("auth", err1)
		NewNetworkError("network", err2)
	}
}