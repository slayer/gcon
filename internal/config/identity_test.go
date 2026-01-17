package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAuthenticatedIdentity_UserCredentials(t *testing.T) {
	// Clear cache before test
	ClearIdentityCache()
	// Create temporary gcloud config directory
	tmpDir := t.TempDir()

	// Create config structure
	configDir := filepath.Join(tmpDir, ".config", "gcloud")
	configsDir := filepath.Join(configDir, "configurations")
	require.NoError(t, os.MkdirAll(configsDir, 0755))

	// Write active_config file
	activeConfigPath := filepath.Join(configDir, "active_config")
	require.NoError(t, os.WriteFile(activeConfigPath, []byte("default\n"), 0644))

	// Write config_default file with account
	configPath := filepath.Join(configsDir, "config_default")
	configContent := `[core]
account = user@example.com
project = test-project
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Set CLOUDSDK_CONFIG to use our temp directory
	oldCloudSDKConfig := os.Getenv("CLOUDSDK_CONFIG")
	t.Cleanup(func() {
		if oldCloudSDKConfig != "" {
			_ = os.Setenv("CLOUDSDK_CONFIG", oldCloudSDKConfig)
		} else {
			_ = os.Unsetenv("CLOUDSDK_CONFIG")
		}
	})
	_ = os.Setenv("CLOUDSDK_CONFIG", configDir)

	// Clear GOOGLE_APPLICATION_CREDENTIALS to ensure gcloud method is used
	oldGoogleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	t.Cleanup(func() {
		if oldGoogleCreds != "" {
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", oldGoogleCreds)
		} else {
			_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
		}
	})
	_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	// Test
	identity, identityType, err := GetAuthenticatedIdentity()
	assert.NoError(t, err)
	assert.Equal(t, "user@example.com", identity)
	assert.Equal(t, IdentityUser, identityType)
}

func TestGetAuthenticatedIdentity_ServiceAccount(t *testing.T) {
	// Clear cache before test
	ClearIdentityCache()
	// Create temporary service account JSON file
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "sa-key.json")
	credContent := `{
  "type": "service_account",
  "project_id": "test-project",
  "private_key_id": "key123",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----\n",
  "client_email": "test-sa@test-project.iam.gserviceaccount.com",
  "client_id": "123456789",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs"
}`
	require.NoError(t, os.WriteFile(credFile, []byte(credContent), 0644))

	// Set GOOGLE_APPLICATION_CREDENTIALS
	oldGoogleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	t.Cleanup(func() {
		if oldGoogleCreds != "" {
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", oldGoogleCreds)
		} else {
			_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
		}
	})
	_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credFile)

	// Clear gcloud config to ensure SA method is used
	oldCloudSDKConfig := os.Getenv("CLOUDSDK_CONFIG")
	t.Cleanup(func() {
		if oldCloudSDKConfig != "" {
			_ = os.Setenv("CLOUDSDK_CONFIG", oldCloudSDKConfig)
		} else {
			_ = os.Unsetenv("CLOUDSDK_CONFIG")
		}
	})
	_ = os.Setenv("CLOUDSDK_CONFIG", filepath.Join(tmpDir, "nonexistent"))

	// Test
	identity, identityType, err := GetAuthenticatedIdentity()
	assert.NoError(t, err)
	assert.Equal(t, "test-sa@test-project.iam.gserviceaccount.com", identity)
	assert.Equal(t, IdentityServiceAccount, identityType)
}

func TestGetAuthenticatedIdentity_NoCredentials(t *testing.T) {
	// Clear cache before test
	ClearIdentityCache()
	// Clear all credential sources
	tmpDir := t.TempDir()

	oldCloudSDKConfig := os.Getenv("CLOUDSDK_CONFIG")
	oldGoogleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	oldCoreAccount := os.Getenv("CLOUDSDK_CORE_ACCOUNT")

	t.Cleanup(func() {
		if oldCloudSDKConfig != "" {
			_ = os.Setenv("CLOUDSDK_CONFIG", oldCloudSDKConfig)
		} else {
			_ = os.Unsetenv("CLOUDSDK_CONFIG")
		}
		if oldGoogleCreds != "" {
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", oldGoogleCreds)
		} else {
			_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
		}
		if oldCoreAccount != "" {
			_ = os.Setenv("CLOUDSDK_CORE_ACCOUNT", oldCoreAccount)
		} else {
			_ = os.Unsetenv("CLOUDSDK_CORE_ACCOUNT")
		}
	})

	_ = os.Setenv("CLOUDSDK_CONFIG", filepath.Join(tmpDir, "nonexistent"))
	_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
	_ = os.Unsetenv("CLOUDSDK_CORE_ACCOUNT")

	// Test
	identity, identityType, err := GetAuthenticatedIdentity()
	assert.Error(t, err)
	assert.Empty(t, identity)
	assert.Equal(t, IdentityUnknown, identityType)
	assert.Contains(t, err.Error(), "unable to determine authenticated identity")
}

func TestExtractServiceAccountEmail_ValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "sa-key.json")
	credContent := `{
  "type": "service_account",
  "client_email": "my-sa@my-project.iam.gserviceaccount.com"
}`
	require.NoError(t, os.WriteFile(credFile, []byte(credContent), 0644))

	email, err := extractServiceAccountEmail(credFile)
	assert.NoError(t, err)
	assert.Equal(t, "my-sa@my-project.iam.gserviceaccount.com", email)
}

func TestExtractServiceAccountEmail_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "bad.json")
	credContent := `{this is not valid json`
	require.NoError(t, os.WriteFile(credFile, []byte(credContent), 0644))

	email, err := extractServiceAccountEmail(credFile)
	assert.Error(t, err)
	assert.Empty(t, email)
	assert.Contains(t, err.Error(), "failed to parse credentials JSON")
}

func TestExtractServiceAccountEmail_MissingEmail(t *testing.T) {
	tmpDir := t.TempDir()
	credFile := filepath.Join(tmpDir, "no-email.json")
	credContent := `{
  "type": "service_account",
  "project_id": "test-project"
}`
	require.NoError(t, os.WriteFile(credFile, []byte(credContent), 0644))

	email, err := extractServiceAccountEmail(credFile)
	assert.Error(t, err)
	assert.Empty(t, email)
	assert.Contains(t, err.Error(), "client_email field is empty or missing")
}

func TestExtractServiceAccountEmail_FileNotFound(t *testing.T) {
	email, err := extractServiceAccountEmail("/nonexistent/file.json")
	assert.Error(t, err)
	assert.Empty(t, email)
	assert.Contains(t, err.Error(), "failed to read credentials file")
}

func TestGetAuthenticatedIdentity_EnvVarAccount(t *testing.T) {
	// Clear cache before test
	ClearIdentityCache()
	// Set CLOUDSDK_CORE_ACCOUNT env var (takes precedence over gcloud config)
	oldCoreAccount := os.Getenv("CLOUDSDK_CORE_ACCOUNT")
	oldGoogleCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	t.Cleanup(func() {
		if oldCoreAccount != "" {
			_ = os.Setenv("CLOUDSDK_CORE_ACCOUNT", oldCoreAccount)
		} else {
			_ = os.Unsetenv("CLOUDSDK_CORE_ACCOUNT")
		}
		if oldGoogleCreds != "" {
			_ = os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", oldGoogleCreds)
		} else {
			_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")
		}
	})

	_ = os.Setenv("CLOUDSDK_CORE_ACCOUNT", "envvar@example.com")
	_ = os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	identity, identityType, err := GetAuthenticatedIdentity()
	assert.NoError(t, err)
	assert.Equal(t, "envvar@example.com", identity)
	assert.Equal(t, IdentityUser, identityType)
}
