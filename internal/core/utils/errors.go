package utils

import "fmt"

type ErrorType int

const (
	ErrorTypeAuth ErrorType = iota
	ErrorTypeNetwork
	ErrorTypeGit
	ErrorTypeFileSystem
	ErrorTypeValidation
	ErrorTypePermission
	ErrorTypeProcessing
)

type GitHubReplaceError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

func (e *GitHubReplaceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *GitHubReplaceError) Unwrap() error {
	return e.Cause
}

func NewAuthError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypeAuth,
		Message: message,
		Cause:   cause,
	}
}

func NewNetworkError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypeNetwork,
		Message: message,
		Cause:   cause,
	}
}

func NewGitError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypeGit,
		Message: message,
		Cause:   cause,
	}
}

func NewFileSystemError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypeFileSystem,
		Message: message,
		Cause:   cause,
	}
}

func NewValidationError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypeValidation,
		Message: message,
		Cause:   cause,
	}
}

func NewPermissionError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypePermission,
		Message: message,
		Cause:   cause,
	}
}

func NewProcessingError(message string, cause error) *GitHubReplaceError {
	return &GitHubReplaceError{
		Type:    ErrorTypeProcessing,
		Message: message,
		Cause:   cause,
	}
}

func (e *GitHubReplaceError) WithContext(key string, value interface{}) *GitHubReplaceError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

func IsAuthError(err error) bool {
	if gre, ok := err.(*GitHubReplaceError); ok {
		return gre.Type == ErrorTypeAuth
	}
	return false
}

func IsNetworkError(err error) bool {
	if gre, ok := err.(*GitHubReplaceError); ok {
		return gre.Type == ErrorTypeNetwork
	}
	return false
}

func IsGitError(err error) bool {
	if gre, ok := err.(*GitHubReplaceError); ok {
		return gre.Type == ErrorTypeGit
	}
	return false
}
