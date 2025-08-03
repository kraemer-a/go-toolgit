package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ConfigSecurity provides encryption and secure storage for sensitive configuration data
type ConfigSecurity struct {
	encryptionKey []byte
}

// NewConfigSecurity creates a new configuration security manager
func NewConfigSecurity() (*ConfigSecurity, error) {
	key, err := getOrCreateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}
	
	return &ConfigSecurity{
		encryptionKey: key,
	}, nil
}

// EncryptToken encrypts a sensitive token for storage
func (cs *ConfigSecurity) EncryptToken(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}
	
	block, err := aes.NewCipher(cs.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Encrypt the token
	ciphertext := gcm.Seal(nonce, nonce, []byte(token), nil)
	
	// Encode to base64 for storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptToken decrypts a stored token
func (cs *ConfigSecurity) DecryptToken(encryptedToken string) (string, error) {
	if encryptedToken == "" {
		return "", fmt.Errorf("encrypted token cannot be empty")
	}
	
	// Decode from base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedToken)
	if err != nil {
		return "", fmt.Errorf("failed to decode token: %w", err)
	}
	
	block, err := aes.NewCipher(cs.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}
	
	if len(ciphertext) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	
	// Extract nonce and ciphertext
	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]
	
	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}
	
	return string(plaintext), nil
}

// SanitizeConfigValue sanitizes configuration values for logging/display
func (cs *ConfigSecurity) SanitizeConfigValue(key, value string) string {
	// List of sensitive keys that should be redacted
	sensitiveKeys := []string{
		"token", "password", "secret", "key", "auth", "credential",
		"github_token", "bitbucket_password", "api_key",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(keyLower, sensitiveKey) {
			if len(value) == 0 {
				return "[EMPTY]"
			}
			if len(value) <= 8 {
				return "[REDACTED]"
			}
			// Show first 4 and last 4 characters with redaction in middle
			return value[:4] + "..." + value[len(value)-4:]
		}
	}
	
	return value
}

// ValidateConfigSecurity validates configuration for security issues
func (cs *ConfigSecurity) ValidateConfigSecurity(config map[string]interface{}) []SecurityIssue {
	var issues []SecurityIssue
	
	for key, value := range config {
		strValue, ok := value.(string)
		if !ok {
			continue
		}
		
		// Check for tokens stored in plain text
		if cs.isSensitiveKey(key) && !cs.isEncrypted(strValue) {
			issues = append(issues, SecurityIssue{
				Type:        "unencrypted_credential",
				Severity:    "high",
				Key:         key,
				Description: "Sensitive credential stored in plain text",
				Recommendation: "Use encrypted storage for sensitive credentials",
			})
		}
		
		// Check for hardcoded secrets
		if cs.containsHardcodedSecret(strValue) {
			issues = append(issues, SecurityIssue{
				Type:        "hardcoded_secret",
				Severity:    "critical",
				Key:         key,
				Description: "Hardcoded secret detected in configuration",
				Recommendation: "Move secrets to environment variables or encrypted storage",
			})
		}
		
		// Check for insecure URLs
		if strings.Contains(key, "url") && strings.HasPrefix(strValue, "http://") {
			issues = append(issues, SecurityIssue{
				Type:        "insecure_url",
				Severity:    "medium",
				Key:         key,
				Description: "HTTP URL used instead of HTTPS",
				Recommendation: "Use HTTPS URLs for secure communication",
			})
		}
	}
	
	return issues
}

// SecurityIssue represents a security problem in configuration
type SecurityIssue struct {
	Type           string `json:"type"`
	Severity       string `json:"severity"`
	Key            string `json:"key"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
}

// GetConfigDirectory returns the secure configuration directory
func (cs *ConfigSecurity) GetConfigDirectory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	configDir := filepath.Join(homeDir, ".go-toolgit")
	
	// Ensure directory exists with secure permissions
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}
	
	return configDir, nil
}

// SecureConfigFile secures the permissions of a configuration file
func (cs *ConfigSecurity) SecureConfigFile(filePath string) error {
	// Set file permissions to be readable only by owner
	if err := os.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("failed to secure config file permissions: %w", err)
	}
	
	return nil
}

// GenerateSecureToken generates a cryptographically secure token
func (cs *ConfigSecurity) GenerateSecureToken(length int) (string, error) {
	if length <= 0 || length > 256 {
		return "", fmt.Errorf("invalid token length: must be between 1 and 256")
	}
	
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	return hex.EncodeToString(bytes), nil
}

// Helper methods

func (cs *ConfigSecurity) isSensitiveKey(key string) bool {
	sensitiveKeys := []string{
		"token", "password", "secret", "key", "auth", "credential",
		"github_token", "bitbucket_password", "api_key",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitiveKey := range sensitiveKeys {
		if strings.Contains(keyLower, sensitiveKey) {
			return true
		}
	}
	
	return false
}

func (cs *ConfigSecurity) isEncrypted(value string) bool {
	// Check if value looks like encrypted data (base64 encoded)
	if _, err := base64.StdEncoding.DecodeString(value); err != nil {
		return false
	}
	
	// Additional heuristics: encrypted values are typically longer than 32 chars
	return len(value) > 32
}

func (cs *ConfigSecurity) containsHardcodedSecret(value string) bool {
	// Common patterns for hardcoded secrets
	hardcodedPatterns := []string{
		"ghp_", "gho_", "ghs_", "ghu_", // GitHub tokens
		"sk_", "pk_", // Stripe keys
		"AIza", // Google API keys
		"AKIA", // AWS access keys
	}
	
	for _, pattern := range hardcodedPatterns {
		if strings.Contains(value, pattern) {
			return true
		}
	}
	
	// Check for obvious test/dummy values
	testValues := []string{
		"test", "dummy", "fake", "sample", "example",
		"12345", "abcdef", "password", "secret",
	}
	
	valueLower := strings.ToLower(value)
	for _, testValue := range testValues {
		if strings.Contains(valueLower, testValue) && len(value) > 10 {
			return true
		}
	}
	
	return false
}

func getOrCreateEncryptionKey() ([]byte, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	
	keyFile := filepath.Join(homeDir, ".go-toolgit", "encryption.key")
	
	// Try to read existing key
	if keyData, err := os.ReadFile(keyFile); err == nil {
		if len(keyData) == 32 { // AES-256 key size
			return keyData, nil
		}
	}
	
	// Generate new key
	key := make([]byte, 32) // AES-256
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(keyFile), 0700); err != nil {
		return nil, fmt.Errorf("failed to create key directory: %w", err)
	}
	
	// Save key with secure permissions
	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		return nil, fmt.Errorf("failed to save encryption key: %w", err)
	}
	
	return key, nil
}

// TokenManager provides secure token management
type TokenManager struct {
	configSecurity *ConfigSecurity
}

// NewTokenManager creates a new token manager
func NewTokenManager() (*TokenManager, error) {
	cs, err := NewConfigSecurity()
	if err != nil {
		return nil, err
	}
	
	return &TokenManager{
		configSecurity: cs,
	}, nil
}

// StoreToken securely stores a token
func (tm *TokenManager) StoreToken(name, token string) error {
	validator := NewInputValidator(true)
	
	// Validate inputs
	if err := validator.ValidateString("name", name, 100); err != nil {
		return fmt.Errorf("invalid token name: %w", err)
	}
	
	if err := validator.ValidateToken("token", token); err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}
	
	// Encrypt token
	encryptedToken, err := tm.configSecurity.EncryptToken(token)
	if err != nil {
		return fmt.Errorf("failed to encrypt token: %w", err)
	}
	
	// Store encrypted token (implementation would depend on storage backend)
	configDir, err := tm.configSecurity.GetConfigDirectory()
	if err != nil {
		return err
	}
	
	tokenFile := filepath.Join(configDir, fmt.Sprintf("token_%s.enc", name))
	if err := os.WriteFile(tokenFile, []byte(encryptedToken), 0600); err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}
	
	return nil
}

// RetrieveToken securely retrieves a stored token
func (tm *TokenManager) RetrieveToken(name string) (string, error) {
	validator := NewInputValidator(true)
	
	// Validate input
	if err := validator.ValidateString("name", name, 100); err != nil {
		return "", fmt.Errorf("invalid token name: %w", err)
	}
	
	configDir, err := tm.configSecurity.GetConfigDirectory()
	if err != nil {
		return "", err
	}
	
	tokenFile := filepath.Join(configDir, fmt.Sprintf("token_%s.enc", name))
	encryptedToken, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("failed to read token file: %w", err)
	}
	
	// Decrypt token
	token, err := tm.configSecurity.DecryptToken(string(encryptedToken))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}
	
	return token, nil
}

// HashPassword creates a secure hash of a password
func HashPassword(password string) (string, error) {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:]), nil
}