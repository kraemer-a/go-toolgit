package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go-toolgit/internal/core/security"
)

// SecureConfigManager handles automatic encryption/decryption of sensitive configuration values
type SecureConfigManager struct {
	configSecurity *security.ConfigSecurity
}

// NewSecureConfigManager creates a new secure configuration manager
func NewSecureConfigManager() (*SecureConfigManager, error) {
	cs, err := security.NewConfigSecurity()
	if err != nil {
		return nil, fmt.Errorf("failed to create config security: %w", err)
	}

	return &SecureConfigManager{
		configSecurity: cs,
	}, nil
}

const encryptionPrefix = "AES256GCM:"

// LoadSecureConfig loads configuration with automatic decryption of sensitive fields
func (scm *SecureConfigManager) LoadSecureConfig() (*Config, error) {
	// Load the regular config first
	config, err := Load()
	if err != nil {
		return nil, err
	}

	// Decrypt sensitive fields
	if err := scm.decryptSensitiveFields(config); err != nil {
		return nil, fmt.Errorf("failed to decrypt sensitive fields: %w", err)
	}

	return config, nil
}

// SaveSecureConfig saves configuration with automatic encryption of sensitive fields
func (scm *SecureConfigManager) SaveSecureConfig(config *Config) error {
	// Create a copy of the config to avoid modifying the original
	configCopy := *config
	
	// Encrypt sensitive fields in the copy
	if err := scm.encryptSensitiveFields(&configCopy); err != nil {
		return fmt.Errorf("failed to encrypt sensitive fields: %w", err)
	}

	// Update viper with encrypted values
	scm.updateViperWithConfig(&configCopy)

	return nil
}

// SaveSecureConfigToFile saves encrypted configuration to a specific file
func (scm *SecureConfigManager) SaveSecureConfigToFile(config *Config, filePath string) error {
	// Create a copy and encrypt sensitive fields
	configCopy := *config
	if err := scm.encryptSensitiveFields(&configCopy); err != nil {
		return fmt.Errorf("failed to encrypt sensitive fields: %w", err)
	}

	// Update viper with encrypted values
	scm.updateViperWithConfig(&configCopy)

	// Write to specific file
	if err := viper.WriteConfigAs(filePath); err != nil {
		return fmt.Errorf("failed to write secure config to file: %w", err)
	}

	// Secure the file permissions
	if err := scm.configSecurity.SecureConfigFile(filePath); err != nil {
		return fmt.Errorf("failed to secure config file: %w", err)
	}

	return nil
}

// decryptSensitiveFields decrypts sensitive fields in the configuration
func (scm *SecureConfigManager) decryptSensitiveFields(config *Config) error {
	// Decrypt GitHub token
	if config.GitHub.Token != "" {
		decrypted, err := scm.decryptIfEncrypted(config.GitHub.Token)
		if err != nil {
			return fmt.Errorf("failed to decrypt GitHub token: %w", err)
		}
		config.GitHub.Token = decrypted
	}

	// Decrypt Bitbucket password
	if config.Bitbucket.Password != "" {
		decrypted, err := scm.decryptIfEncrypted(config.Bitbucket.Password)
		if err != nil {
			return fmt.Errorf("failed to decrypt Bitbucket password: %w", err)
		}
		config.Bitbucket.Password = decrypted
	}

	return nil
}

// encryptSensitiveFields encrypts sensitive fields in the configuration
func (scm *SecureConfigManager) encryptSensitiveFields(config *Config) error {
	// Encrypt GitHub token
	if config.GitHub.Token != "" && !scm.isEncrypted(config.GitHub.Token) {
		encrypted, err := scm.encryptValue(config.GitHub.Token)
		if err != nil {
			return fmt.Errorf("failed to encrypt GitHub token: %w", err)
		}
		config.GitHub.Token = encrypted
	}

	// Encrypt Bitbucket password
	if config.Bitbucket.Password != "" && !scm.isEncrypted(config.Bitbucket.Password) {
		encrypted, err := scm.encryptValue(config.Bitbucket.Password)
		if err != nil {
			return fmt.Errorf("failed to encrypt Bitbucket password: %w", err)
		}
		config.Bitbucket.Password = encrypted
	}

	return nil
}

// decryptIfEncrypted decrypts a value if it's encrypted, otherwise returns it as-is
func (scm *SecureConfigManager) decryptIfEncrypted(value string) (string, error) {
	if !scm.isEncrypted(value) {
		// Value is not encrypted, return as-is
		return value, nil
	}

	// Remove the encryption prefix and decrypt
	encryptedValue := strings.TrimPrefix(value, encryptionPrefix)
	decrypted, err := scm.configSecurity.DecryptToken(encryptedValue)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt value: %w", err)
	}

	return decrypted, nil
}

// encryptValue encrypts a plain text value and adds the encryption prefix
func (scm *SecureConfigManager) encryptValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	encrypted, err := scm.configSecurity.EncryptToken(value)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt value: %w", err)
	}

	return encryptionPrefix + encrypted, nil
}

// isEncrypted checks if a value is encrypted by looking for the encryption prefix
func (scm *SecureConfigManager) isEncrypted(value string) bool {
	return strings.HasPrefix(value, encryptionPrefix)
}

// updateViperWithConfig updates viper with configuration values
func (scm *SecureConfigManager) updateViperWithConfig(config *Config) {
	// Update all configuration values in viper
	viper.Set("provider", config.Provider)
	
	// GitHub config
	viper.Set("github.base_url", config.GitHub.BaseURL)
	viper.Set("github.token", config.GitHub.Token)
	viper.Set("github.organization", config.GitHub.Org)
	viper.Set("github.team", config.GitHub.Team)
	viper.Set("github.timeout", config.GitHub.Timeout)
	viper.Set("github.max_retries", config.GitHub.MaxRetries)
	viper.Set("github.wait_for_rate_limit", config.GitHub.WaitForRateLimit)
	
	// Bitbucket config
	viper.Set("bitbucket.base_url", config.Bitbucket.BaseURL)
	viper.Set("bitbucket.username", config.Bitbucket.Username)
	viper.Set("bitbucket.password", config.Bitbucket.Password)
	viper.Set("bitbucket.project", config.Bitbucket.Project)
	viper.Set("bitbucket.timeout", config.Bitbucket.Timeout)
	viper.Set("bitbucket.max_retries", config.Bitbucket.MaxRetries)
	
	// Processing config
	viper.Set("processing.include_patterns", config.Processing.IncludePatterns)
	viper.Set("processing.exclude_patterns", config.Processing.ExcludePatterns)
	viper.Set("processing.max_workers", config.Processing.MaxWorkers)
	
	// Pull request config
	viper.Set("pull_request.title_template", config.PullRequest.TitleTemplate)
	viper.Set("pull_request.body_template", config.PullRequest.BodyTemplate)
	viper.Set("pull_request.branch_prefix", config.PullRequest.BranchPrefix)
	viper.Set("pull_request.auto_merge", config.PullRequest.AutoMerge)
	viper.Set("pull_request.delete_branch", config.PullRequest.DeleteBranch)
	
	// Logging config
	viper.Set("logging.level", config.Logging.Level)
	viper.Set("logging.format", config.Logging.Format)
}

// SanitizeConfigForLogging returns a sanitized version of the config for logging
func (scm *SecureConfigManager) SanitizeConfigForLogging(config *Config) *Config {
	sanitized := *config
	
	// Sanitize sensitive fields
	sanitized.GitHub.Token = scm.configSecurity.SanitizeConfigValue("github.token", config.GitHub.Token)
	sanitized.Bitbucket.Password = scm.configSecurity.SanitizeConfigValue("bitbucket.password", config.Bitbucket.Password)
	
	return &sanitized
}

// ValidateConfigSecurity validates the configuration for security issues
func (scm *SecureConfigManager) ValidateConfigSecurity(config *Config) []security.SecurityIssue {
	configMap := make(map[string]interface{})
	
	// Convert config to map for validation
	configMap["github.token"] = config.GitHub.Token
	configMap["bitbucket.password"] = config.Bitbucket.Password
	configMap["github.base_url"] = config.GitHub.BaseURL
	configMap["bitbucket.base_url"] = config.Bitbucket.BaseURL
	
	return scm.configSecurity.ValidateConfigSecurity(configMap)
}

// IsCredentialEncrypted checks if a specific credential is encrypted
func (scm *SecureConfigManager) IsCredentialEncrypted(value string) bool {
	return scm.isEncrypted(value)
}