package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigSecurity_EncryptDecryptToken(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	originalToken := "ghp_test_token_123456789abcdef"
	
	// Encrypt token
	encrypted, err := cs.EncryptToken(originalToken)
	if err != nil {
		t.Fatalf("Failed to encrypt token: %v", err)
	}
	
	if encrypted == originalToken {
		t.Error("Encrypted token should not match original")
	}
	
	// Decrypt token
	decrypted, err := cs.DecryptToken(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt token: %v", err)
	}
	
	if decrypted != originalToken {
		t.Errorf("Decrypted token doesn't match original: got %s, want %s", decrypted, originalToken)
	}
}

func TestConfigSecurity_SanitizeConfigValue(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "regular value",
			key:      "url",
			value:    "https://api.github.com",
			expected: "https://api.github.com",
		},
		{
			name:     "token value",
			key:      "github_token",
			value:    "ghp_1234567890abcdef",
			expected: "ghp_...cdef",
		},
		{
			name:     "password value",
			key:      "password",
			value:    "secretpassword123",
			expected: "secr...d123",
		},
		{
			name:     "short sensitive value",
			key:      "secret",
			value:    "short",
			expected: "[REDACTED]",
		},
		{
			name:     "empty sensitive value",
			key:      "api_key",
			value:    "",
			expected: "[EMPTY]",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cs.SanitizeConfigValue(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("SanitizeConfigValue() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestConfigSecurity_ValidateConfigSecurity(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	config := map[string]interface{}{
		"github_token":     "ghp_hardcoded_token_123",    // Hardcoded secret
		"password":         "plaintext_password",         // Unencrypted credential
		"base_url":         "http://insecure.com",        // Insecure URL
		"safe_setting":     "normal_value",               // Safe value
		"encrypted_token":  "dGVzdGVuY3J5cHRlZHRva2VuMTIzNDU2Nzg5YWJjZGVmZ2hpamtsbW5vcA==", // Looks encrypted
	}
	
	issues := cs.ValidateConfigSecurity(config)
	
	if len(issues) == 0 {
		t.Error("Expected security issues to be found")
	}
	
	// Check for specific issue types
	hasHardcodedSecret := false
	hasUnencryptedCredential := false
	hasInsecureURL := false
	
	for _, issue := range issues {
		switch issue.Type {
		case "hardcoded_secret":
			hasHardcodedSecret = true
		case "unencrypted_credential":
			hasUnencryptedCredential = true
		case "insecure_url":
			hasInsecureURL = true
		}
	}
	
	if !hasHardcodedSecret {
		t.Error("Expected hardcoded secret issue to be detected")
	}
	
	if !hasUnencryptedCredential {
		t.Error("Expected unencrypted credential issue to be detected")
	}
	
	if !hasInsecureURL {
		t.Error("Expected insecure URL issue to be detected")
	}
}

func TestConfigSecurity_GenerateSecureToken(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	tests := []struct {
		name        string
		length      int
		expectError bool
	}{
		{
			name:        "valid length",
			length:      16,
			expectError: false,
		},
		{
			name:        "zero length",
			length:      0,
			expectError: true,
		},
		{
			name:        "negative length",
			length:      -1,
			expectError: true,
		},
		{
			name:        "excessive length",
			length:      1000,
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := cs.GenerateSecureToken(tt.length)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Check token properties
			if len(token) != tt.length*2 { // Hex encoding doubles length
				t.Errorf("Expected token length %d, got %d", tt.length*2, len(token))
			}
			
			// Verify it's valid hex
			for _, char := range token {
				if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
					t.Errorf("Token contains invalid hex character: %c", char)
				}
			}
		})
	}
}

func TestConfigSecurity_GetConfigDirectory(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	configDir, err := cs.GetConfigDirectory()
	if err != nil {
		t.Fatalf("Failed to get config directory: %v", err)
	}
	
	// Check that directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("Config directory was not created")
	}
	
	// Check directory name
	if !strings.HasSuffix(configDir, ".go-toolgit") {
		t.Errorf("Expected directory to end with '.go-toolgit', got: %s", configDir)
	}
	
	// Check directory permissions
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("Failed to stat config directory: %v", err)
	}
	
	if info.Mode().Perm() != 0700 {
		t.Errorf("Expected directory permissions 0700, got %o", info.Mode().Perm())
	}
}

func TestConfigSecurity_SecureConfigFile(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	// Create temporary file
	tmpFile := filepath.Join(t.TempDir(), "test_config.yaml")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Secure the file
	if err := cs.SecureConfigFile(tmpFile); err != nil {
		t.Fatalf("Failed to secure config file: %v", err)
	}
	
	// Check file permissions
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat secured file: %v", err)
	}
	
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestTokenManager_StoreRetrieveToken(t *testing.T) {
	tm, err := NewTokenManager()
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}
	
	tokenName := "test_token"
	originalToken := "ghp_test_token_123456789abcdef"
	
	// Store token
	if err := tm.StoreToken(tokenName, originalToken); err != nil {
		t.Fatalf("Failed to store token: %v", err)
	}
	
	// Retrieve token
	retrievedToken, err := tm.RetrieveToken(tokenName)
	if err != nil {
		t.Fatalf("Failed to retrieve token: %v", err)
	}
	
	if retrievedToken != originalToken {
		t.Errorf("Retrieved token doesn't match original: got %s, want %s", retrievedToken, originalToken)
	}
}

func TestTokenManager_ValidatesInputs(t *testing.T) {
	tm, err := NewTokenManager()
	if err != nil {
		t.Fatalf("Failed to create token manager: %v", err)
	}
	
	// Test invalid token name
	err = tm.StoreToken("name; rm -rf /", "ghp_valid_token_123456789")
	if err == nil {
		t.Error("Expected token manager to reject malicious token name")
	}
	
	// Test invalid token
	err = tm.StoreToken("valid_name", "invalid; rm -rf /")
	if err == nil {
		t.Error("Expected token manager to reject malicious token")
	}
	
	// Test retrieval with invalid name
	_, err = tm.RetrieveToken("../../../etc/passwd")
	if err == nil {
		t.Error("Expected token manager to reject malicious retrieval name")
	}
}

func TestHashPassword(t *testing.T) {
	password := "test_password_123"
	
	hash1, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	
	hash2, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password second time: %v", err)
	}
	
	// Same password should produce same hash
	if hash1 != hash2 {
		t.Error("Same password produced different hashes")
	}
	
	// Hash should be different from original password
	if hash1 == password {
		t.Error("Hash should not match original password")
	}
	
	// Hash should be hex encoded
	if len(hash1) != 64 { // SHA256 hex encoding is 64 characters
		t.Errorf("Expected hash length 64, got %d", len(hash1))
	}
}

func TestConfigSecurity_EdgeCases(t *testing.T) {
	cs, err := NewConfigSecurity()
	if err != nil {
		t.Fatalf("Failed to create config security: %v", err)
	}
	
	// Test empty token encryption
	_, err = cs.EncryptToken("")
	if err == nil {
		t.Error("Expected error when encrypting empty token")
	}
	
	// Test empty token decryption
	_, err = cs.DecryptToken("")
	if err == nil {
		t.Error("Expected error when decrypting empty token")
	}
	
	// Test invalid base64 decryption
	_, err = cs.DecryptToken("invalid_base64!")
	if err == nil {
		t.Error("Expected error when decrypting invalid base64")
	}
	
	// Test short ciphertext
	_, err = cs.DecryptToken("dGVzdA==") // "test" in base64, too short
	if err == nil {
		t.Error("Expected error when decrypting short ciphertext")
	}
}

// Benchmark tests
func BenchmarkConfigSecurity_EncryptToken(b *testing.B) {
	cs, _ := NewConfigSecurity()
	token := "ghp_test_token_123456789abcdef"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.EncryptToken(token)
	}
}

func BenchmarkConfigSecurity_DecryptToken(b *testing.B) {
	cs, _ := NewConfigSecurity()
	token := "ghp_test_token_123456789abcdef"
	encrypted, _ := cs.EncryptToken(token)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.DecryptToken(encrypted)
	}
}

func BenchmarkConfigSecurity_ValidateConfig(b *testing.B) {
	cs, _ := NewConfigSecurity()
	config := map[string]interface{}{
		"github_token": "ghp_hardcoded_token_123",
		"password":     "plaintext_password",
		"base_url":     "http://insecure.com",
		"safe_setting": "normal_value",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs.ValidateConfigSecurity(config)
	}
}