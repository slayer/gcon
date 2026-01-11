package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]map[string]string
	}{
		{
			name: "basic config",
			content: `[core]
account = user@example.com
project = my-project
`,
			expected: map[string]map[string]string{
				"core": {
					"account": "user@example.com",
					"project": "my-project",
				},
			},
		},
		{
			name: "multiple sections",
			content: `[core]
project = my-project

[compute]
region = us-central1
zone = us-central1-a
`,
			expected: map[string]map[string]string{
				"core": {
					"project": "my-project",
				},
				"compute": {
					"region": "us-central1",
					"zone":   "us-central1-a",
				},
			},
		},
		{
			name: "with comments and empty lines",
			content: `# This is a comment
[core]
; Another comment
project = my-project

`,
			expected: map[string]map[string]string{
				"core": {
					"project": "my-project",
				},
			},
		},
		{
			name: "values with spaces",
			content: `[core]
project = my-project-id
account = user@example.com
`,
			expected: map[string]map[string]string{
				"core": {
					"project": "my-project-id",
					"account": "user@example.com",
				},
			},
		},
		{
			name:     "empty file",
			content:  "",
			expected: map[string]map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := createTempFile(t, tt.content)
			defer func() { _ = os.Remove(tmpFile) }()

			result, err := parseConfigFile(tmpFile)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseConfigFile_NonExistent(t *testing.T) {
	_, err := parseConfigFile("/nonexistent/file")
	assert.True(t, os.IsNotExist(err))
}

func TestGetConfigDir(t *testing.T) {
	t.Run("uses CLOUDSDK_CONFIG if set", func(t *testing.T) {
		customDir := "/custom/gcloud/config"
		t.Setenv("CLOUDSDK_CONFIG", customDir)

		result := getConfigDir()
		assert.Equal(t, customDir, result)
	})

	t.Run("falls back to default location", func(t *testing.T) {
		t.Setenv("CLOUDSDK_CONFIG", "")

		result := getConfigDir()

		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".config", "gcloud")
		assert.Equal(t, expected, result)
	})
}

func TestLoadGcloudConfig(t *testing.T) {
	t.Run("loads config from active configuration", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)

		// Create properties file indicating active config
		propertiesContent := `[core]
config = prod
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "properties"), []byte(propertiesContent), 0644))

		// Create configurations directory and config file
		configDir := filepath.Join(tmpDir, "configurations")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		configContent := `[core]
account = user@example.com
project = production-project
`
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "config_prod"), []byte(configContent), 0644))

		config, err := LoadGcloudConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "prod", config.ActiveConfig)
		assert.Equal(t, "production-project", config.Project)
		assert.Equal(t, "user@example.com", config.Account)
	})

	t.Run("uses default config when no active config specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)

		// Create configurations directory and default config file
		configDir := filepath.Join(tmpDir, "configurations")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		configContent := `[core]
project = default-project
`
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "config_default"), []byte(configContent), 0644))

		config, err := LoadGcloudConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "default", config.ActiveConfig)
		assert.Equal(t, "default-project", config.Project)
	})

	t.Run("returns nil when config dir does not exist", func(t *testing.T) {
		t.Setenv("CLOUDSDK_CONFIG", "/nonexistent/gcloud/config")

		config, err := LoadGcloudConfig()
		require.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("returns nil when config file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)
		t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "")

		// Create configurations directory but no config files
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "configurations"), 0755))

		config, err := LoadGcloudConfig()
		require.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("CLOUDSDK_ACTIVE_CONFIG_NAME overrides properties file", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)
		t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "staging")

		// Create properties file indicating different active config
		propertiesContent := `[core]
config = prod
`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "properties"), []byte(propertiesContent), 0644))

		// Create configurations directory with both configs
		configDir := filepath.Join(tmpDir, "configurations")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		require.NoError(t, os.WriteFile(
			filepath.Join(configDir, "config_prod"),
			[]byte("[core]\nproject = prod-project\n"),
			0644,
		))
		require.NoError(t, os.WriteFile(
			filepath.Join(configDir, "config_staging"),
			[]byte("[core]\nproject = staging-project\n"),
			0644,
		))

		config, err := LoadGcloudConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		// Should use staging (from env) not prod (from properties)
		assert.Equal(t, "staging", config.ActiveConfig)
		assert.Equal(t, "staging-project", config.Project)
	})

	t.Run("loads zone and region from compute section", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)
		t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "")

		configDir := filepath.Join(tmpDir, "configurations")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		configContent := `[core]
project = my-project

[compute]
zone = us-central1-a
region = us-central1
`
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "config_default"), []byte(configContent), 0644))

		config, err := LoadGcloudConfig()
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.Equal(t, "us-central1-a", config.Zone)
		assert.Equal(t, "us-central1", config.Region)
	})
}

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "gcloud_config_test_*")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	return tmpFile.Name()
}
