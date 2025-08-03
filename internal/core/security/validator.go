package security

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Input validation limits
const (
	MaxInputLength        = 10000   // Maximum length for most user inputs
	MaxTokenLength        = 1000    // Maximum length for API tokens
	MaxURLLength          = 2000    // Maximum length for URLs
	MaxFilePathLength     = 1000    // Maximum length for file paths
	MaxBranchNameLength   = 250     // Maximum length for git branch names
	MaxCommitMsgLength    = 5000    // Maximum length for commit messages
	MaxSearchQueryLength  = 1000    // Maximum length for search queries
	MaxReplacementLength  = 5000    // Maximum length for replacement strings
)

// Dangerous patterns that should be blocked
var (
	commandInjectionPatterns = []string{
		";", "&", "|", "`", "$", "$(", ")", "&&", "||",
		"rm ", "del ", "curl ", "wget ", "nc ", "telnet ",
		"bash", "sh ", "cmd", "powershell", "exec",
	}
	
	scriptInjectionPatterns = []string{
		"<script", "</script", "javascript:", "vbscript:",
		"onload", "onerror", "onclick", "eval(",
		"document.", "window.", "alert(",
	}
	
	pathTraversalPatterns = []string{
		"../", "..\\", "..", "%2e%2e", "%252e%252e",
		"/..", "\\..", "....//", "....\\\\",
	}
	
	sqlInjectionPatterns = []string{
		"'", "\"", "--", "/*", "*/", "union", "select",
		"insert", "update", "delete", "drop", "create",
		"alter", "exec", "sp_", "xp_",
	}
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
	Value   string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// InputValidator provides methods for validating user inputs
type InputValidator struct {
	strictMode bool
}

// NewInputValidator creates a new input validator
func NewInputValidator(strictMode bool) *InputValidator {
	return &InputValidator{
		strictMode: strictMode,
	}
}

// ValidateString validates a general string input
func (v *InputValidator) ValidateString(field, value string, maxLength int) error {
	if err := v.checkLength(field, value, maxLength); err != nil {
		return err
	}
	
	if err := v.checkDangerousPatterns(field, value); err != nil {
		return err
	}
	
	if err := v.checkEncoding(field, value); err != nil {
		return err
	}
	
	return nil
}

// ValidateToken validates API tokens and credentials
func (v *InputValidator) ValidateToken(field, token string) error {
	if token == "" {
		return ValidationError{Field: field, Message: "token cannot be empty"}
	}
	
	if err := v.checkLength(field, token, MaxTokenLength); err != nil {
		return err
	}
	
	// Check for suspicious patterns in tokens
	tokenLower := strings.ToLower(token)
	for _, pattern := range commandInjectionPatterns {
		if strings.Contains(tokenLower, pattern) {
			return ValidationError{
				Field:   field,
				Message: fmt.Sprintf("token contains suspicious pattern: %s", pattern),
				Value:   "[REDACTED]",
			}
		}
	}
	
	// Basic format validation for GitHub tokens
	if strings.HasPrefix(token, "ghp_") || strings.HasPrefix(token, "gho_") {
		if len(token) < 20 {
			return ValidationError{Field: field, Message: "GitHub token appears to be too short"}
		}
	}
	
	return nil
}

// ValidateURL validates URLs for safety
func (v *InputValidator) ValidateURL(field, urlStr string) error {
	if urlStr == "" {
		return ValidationError{Field: field, Message: "URL cannot be empty"}
	}
	
	if err := v.checkLength(field, urlStr, MaxURLLength); err != nil {
		return err
	}
	
	// Parse URL to validate structure
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ValidationError{Field: field, Message: "invalid URL format", Value: urlStr}
	}
	
	// Check scheme
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return ValidationError{Field: field, Message: "URL must use http or https scheme", Value: urlStr}
	}
	
	// Check for dangerous patterns
	if err := v.checkDangerousPatterns(field, urlStr); err != nil {
		return err
	}
	
	// Check for path traversal in URL path
	if strings.Contains(parsedURL.Path, "..") {
		return ValidationError{Field: field, Message: "URL contains path traversal attempt", Value: urlStr}
	}
	
	return nil
}

// ValidateFilePath validates file paths for safety
func (v *InputValidator) ValidateFilePath(field, path string) error {
	if path == "" {
		return ValidationError{Field: field, Message: "file path cannot be empty"}
	}
	
	if err := v.checkLength(field, path, MaxFilePathLength); err != nil {
		return err
	}
	
	// Check for path traversal
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return ValidationError{Field: field, Message: "path contains traversal attempt", Value: path}
	}
	
	// Check for absolute paths that could be dangerous
	isAbsolute := filepath.IsAbs(path)
	isWindowsPath := len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
	
	if isAbsolute || isWindowsPath {
		dangerousPaths := []string{
			"/etc", "/usr", "/bin", "/sbin", "/var", "/root", "/home",
			"C:\\Windows", "C:\\Program Files", "C:\\Users", "C:\\System32",
		}
		
		pathLower := strings.ToLower(path)
		for _, dangerous := range dangerousPaths {
			dangerousLower := strings.ToLower(dangerous)
			if pathLower == dangerousLower || strings.HasPrefix(pathLower, dangerousLower+"\\") || strings.HasPrefix(pathLower, dangerousLower+"/") {
				return ValidationError{Field: field, Message: "path accesses protected directory", Value: path}
			}
		}
	}
	
	// Check for dangerous patterns
	if err := v.checkDangerousPatterns(field, path); err != nil {
		return err
	}
	
	return nil
}

// ValidateBranchName validates git branch names
func (v *InputValidator) ValidateBranchName(field, branchName string) error {
	if branchName == "" {
		return ValidationError{Field: field, Message: "branch name cannot be empty"}
	}
	
	if err := v.checkLength(field, branchName, MaxBranchNameLength); err != nil {
		return err
	}
	
	// Git branch name rules
	invalidChars := []string{" ", "~", "^", ":", "?", "*", "[", "\\", "..", "//"}
	for _, char := range invalidChars {
		if strings.Contains(branchName, char) {
			return ValidationError{Field: field, Message: fmt.Sprintf("branch name contains invalid character: %s", char), Value: branchName}
		}
	}
	
	// Cannot start or end with certain characters
	if strings.HasPrefix(branchName, "/") || strings.HasSuffix(branchName, "/") ||
	   strings.HasPrefix(branchName, ".") || strings.HasSuffix(branchName, ".") {
		return ValidationError{Field: field, Message: "branch name has invalid prefix/suffix", Value: branchName}
	}
	
	// Check for dangerous patterns
	if err := v.checkDangerousPatterns(field, branchName); err != nil {
		return err
	}
	
	return nil
}

// ValidateSearchQuery validates search queries for injection attacks
func (v *InputValidator) ValidateSearchQuery(field, query string) error {
	if err := v.checkLength(field, query, MaxSearchQueryLength); err != nil {
		return err
	}
	
	// Check for dangerous patterns
	if err := v.checkDangerousPatterns(field, query); err != nil {
		return err
	}
	
	// Check for potential ReDoS patterns
	if v.checkReDoSPatterns(query) {
		return ValidationError{Field: field, Message: "query contains potentially dangerous regex patterns", Value: query}
	}
	
	return nil
}

// ValidateReplacement validates replacement strings
func (v *InputValidator) ValidateReplacement(field, replacement string) error {
	if err := v.checkLength(field, replacement, MaxReplacementLength); err != nil {
		return err
	}
	
	// Check for dangerous patterns
	if err := v.checkDangerousPatterns(field, replacement); err != nil {
		return err
	}
	
	return nil
}

// SanitizeString sanitizes a string by removing dangerous characters
func (v *InputValidator) SanitizeString(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	
	// Remove control characters except tab, newline, carriage return
	var result strings.Builder
	for _, r := range input {
		if r >= 32 || r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// Helper methods

func (v *InputValidator) checkLength(field, value string, maxLength int) error {
	if len(value) > maxLength {
		return ValidationError{
			Field:   field,
			Message: fmt.Sprintf("value exceeds maximum length of %d characters", maxLength),
			Value:   value[:min(len(value), 100)] + "...",
		}
	}
	return nil
}

func (v *InputValidator) checkDangerousPatterns(field, value string) error {
	valueLower := strings.ToLower(value)
	
	// Check for command injection
	for _, pattern := range commandInjectionPatterns {
		if strings.Contains(valueLower, pattern) {
			return ValidationError{
				Field:   field,
				Message: fmt.Sprintf("contains potentially dangerous pattern: %s", pattern),
				Value:   value,
			}
		}
	}
	
	// Check for script injection
	for _, pattern := range scriptInjectionPatterns {
		if strings.Contains(valueLower, pattern) {
			return ValidationError{
				Field:   field,
				Message: fmt.Sprintf("contains script injection pattern: %s", pattern),
				Value:   value,
			}
		}
	}
	
	// Check for path traversal
	for _, pattern := range pathTraversalPatterns {
		if strings.Contains(valueLower, pattern) {
			return ValidationError{
				Field:   field,
				Message: fmt.Sprintf("contains path traversal pattern: %s", pattern),
				Value:   value,
			}
		}
	}
	
	// In strict mode, also check for SQL injection patterns
	if v.strictMode {
		for _, pattern := range sqlInjectionPatterns {
			if strings.Contains(valueLower, pattern) {
				return ValidationError{
					Field:   field,
					Message: fmt.Sprintf("contains SQL injection pattern: %s", pattern),
					Value:   value,
				}
			}
		}
	}
	
	return nil
}

func (v *InputValidator) checkEncoding(field, value string) error {
	// Check for valid UTF-8
	if !utf8.ValidString(value) {
		return ValidationError{Field: field, Message: "contains invalid UTF-8 encoding"}
	}
	
	return nil
}

func (v *InputValidator) checkReDoSPatterns(value string) bool {
	// Check for patterns that could cause ReDoS attacks
	redosPatterns := []regexp.Regexp{
		*regexp.MustCompile(`\([^)]*\+[^)]*\)+`),        // (a+)+
		*regexp.MustCompile(`\([^)]*\*[^)]*\)+`),        // (a*)+
		*regexp.MustCompile(`\([^)]*\{[^}]*,[^}]*\}[^)]*\)+`), // (a{1,})+
	}
	
	for _, pattern := range redosPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ValidateConfig validates configuration values
func (v *InputValidator) ValidateConfig(config interface{}) error {
	// This would be implemented based on specific configuration struct
	// For now, return nil as placeholder
	return nil
}

// IsAllowedFileExtension checks if a file extension is allowed
func (v *InputValidator) IsAllowedFileExtension(filename string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		return true // No restrictions
	}
	
	ext := strings.ToLower(filepath.Ext(filename))
	for _, allowed := range allowedExtensions {
		if strings.ToLower(allowed) == ext {
			return true
		}
	}
	
	return false
}