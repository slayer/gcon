package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveProject(t *testing.T) {
	t.Run("CLI flag takes highest priority", func(t *testing.T) {
		t.Setenv("CLOUDSDK_CORE_PROJECT", "env-project")

		// Even with env var set, flag should win
		result := ResolveProject("flag-project")
		assert.Equal(t, "flag-project", result)
	})

	t.Run("env var takes priority over gcloud config", func(t *testing.T) {
		// Set up gcloud config
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)
		t.Setenv("CLOUDSDK_CORE_PROJECT", "env-project")

		configDir := filepath.Join(tmpDir, "configurations")
		require.NoError(t, os.MkdirAll(configDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(configDir, "config_default"),
			[]byte("[core]\nproject = config-project\n"),
			0644,
		))

		result := ResolveProject("")
		assert.Equal(t, "env-project", result)
	})

	t.Run("falls back to gcloud config", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("CLOUDSDK_CONFIG", tmpDir)
		t.Setenv("CLOUDSDK_CORE_PROJECT", "")

		configDir := filepath.Join(tmpDir, "configurations")
		require.NoError(t, os.MkdirAll(configDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(configDir, "config_default"),
			[]byte("[core]\nproject = config-project\n"),
			0644,
		))

		result := ResolveProject("")
		assert.Equal(t, "config-project", result)
	})

	t.Run("returns empty when no config available", func(t *testing.T) {
		t.Setenv("CLOUDSDK_CONFIG", "/nonexistent/path")
		t.Setenv("CLOUDSDK_CORE_PROJECT", "")

		result := ResolveProject("")
		assert.Equal(t, "", result)
	})
}

func TestResolveZone(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		envValue  string
		gcloudVal string
		expected  string
	}{
		{
			name:      "flag takes priority",
			flagValue: "us-central1-a",
			envValue:  "us-east1-b",
			gcloudVal: "europe-west1-c",
			expected:  "us-central1-a",
		},
		{
			name:      "env takes priority over gcloud",
			flagValue: "",
			envValue:  "us-east1-b",
			gcloudVal: "europe-west1-c",
			expected:  "us-east1-b",
		},
		{
			name:      "falls back to gcloud config",
			flagValue: "",
			envValue:  "",
			gcloudVal: "europe-west1-c",
			expected:  "europe-west1-c",
		},
		{
			name:      "returns empty when nothing set",
			flagValue: "",
			envValue:  "",
			gcloudVal: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("CLOUDSDK_CONFIG", tmpDir)
			t.Setenv("CLOUDSDK_COMPUTE_ZONE", tt.envValue)
			t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "")

			if tt.gcloudVal != "" {
				configDir := filepath.Join(tmpDir, "configurations")
				require.NoError(t, os.MkdirAll(configDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(configDir, "config_default"),
					[]byte("[compute]\nzone = "+tt.gcloudVal+"\n"),
					0644,
				))
			}

			result := ResolveZone(tt.flagValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveRegion(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		envValue  string
		gcloudVal string
		expected  string
	}{
		{
			name:      "flag takes priority",
			flagValue: "us-central1",
			envValue:  "us-east1",
			gcloudVal: "europe-west1",
			expected:  "us-central1",
		},
		{
			name:      "env takes priority over gcloud",
			flagValue: "",
			envValue:  "us-east1",
			gcloudVal: "europe-west1",
			expected:  "us-east1",
		},
		{
			name:      "falls back to gcloud config",
			flagValue: "",
			envValue:  "",
			gcloudVal: "europe-west1",
			expected:  "europe-west1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("CLOUDSDK_CONFIG", tmpDir)
			t.Setenv("CLOUDSDK_COMPUTE_REGION", tt.envValue)
			t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "")

			if tt.gcloudVal != "" {
				configDir := filepath.Join(tmpDir, "configurations")
				require.NoError(t, os.MkdirAll(configDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(configDir, "config_default"),
					[]byte("[compute]\nregion = "+tt.gcloudVal+"\n"),
					0644,
				))
			}

			result := ResolveRegion(tt.flagValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveAccount(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		envValue  string
		gcloudVal string
		expected  string
	}{
		{
			name:      "flag takes priority",
			flagValue: "flag@example.com",
			envValue:  "env@example.com",
			gcloudVal: "config@example.com",
			expected:  "flag@example.com",
		},
		{
			name:      "env takes priority over gcloud",
			flagValue: "",
			envValue:  "env@example.com",
			gcloudVal: "config@example.com",
			expected:  "env@example.com",
		},
		{
			name:      "falls back to gcloud config",
			flagValue: "",
			envValue:  "",
			gcloudVal: "config@example.com",
			expected:  "config@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("CLOUDSDK_CONFIG", tmpDir)
			t.Setenv("CLOUDSDK_CORE_ACCOUNT", tt.envValue)
			t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "")

			if tt.gcloudVal != "" {
				configDir := filepath.Join(tmpDir, "configurations")
				require.NoError(t, os.MkdirAll(configDir, 0755))
				require.NoError(t, os.WriteFile(
					filepath.Join(configDir, "config_default"),
					[]byte("[core]\naccount = "+tt.gcloudVal+"\n"),
					0644,
				))
			}

			result := ResolveAccount(tt.flagValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveActiveConfigName(t *testing.T) {
	t.Run("returns env var when set", func(t *testing.T) {
		t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "my-config")
		result := ResolveActiveConfigName()
		assert.Equal(t, "my-config", result)
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		t.Setenv("CLOUDSDK_ACTIVE_CONFIG_NAME", "")
		result := ResolveActiveConfigName()
		assert.Equal(t, "", result)
	})
}

func TestResolveCredentialsFile(t *testing.T) {
	t.Run("returns credentials path when set", func(t *testing.T) {
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/path/to/creds.json")
		result := ResolveCredentialsFile()
		assert.Equal(t, "/path/to/creds.json", result)
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
		result := ResolveCredentialsFile()
		assert.Equal(t, "", result)
	})
}

func TestNoColor(t *testing.T) {
	t.Run("returns true when NO_COLOR is set", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		assert.True(t, NoColor())
	})

	t.Run("returns true when NO_COLOR is empty string", func(t *testing.T) {
		// NO_COLOR standard: presence of the var matters, not its value
		t.Setenv("NO_COLOR", "")
		assert.True(t, NoColor())
	})

	t.Run("returns false when NO_COLOR is unset", func(t *testing.T) {
		_ = os.Unsetenv("NO_COLOR")
		assert.False(t, NoColor())
	})
}

func TestResolveProxy(t *testing.T) {
	t.Run("reads lowercase env vars", func(t *testing.T) {
		t.Setenv("http_proxy", "http://proxy:8080")
		t.Setenv("https_proxy", "https://proxy:8443")
		t.Setenv("no_proxy", "localhost,127.0.0.1")
		t.Setenv("HTTP_PROXY", "")
		t.Setenv("HTTPS_PROXY", "")
		t.Setenv("NO_PROXY", "")

		cfg := ResolveProxy()
		assert.Equal(t, "http://proxy:8080", cfg.HTTPProxy)
		assert.Equal(t, "https://proxy:8443", cfg.HTTPSProxy)
		assert.Equal(t, "localhost,127.0.0.1", cfg.NoProxy)
	})

	t.Run("falls back to uppercase env vars", func(t *testing.T) {
		t.Setenv("http_proxy", "")
		t.Setenv("https_proxy", "")
		t.Setenv("no_proxy", "")
		t.Setenv("HTTP_PROXY", "http://PROXY:8080")
		t.Setenv("HTTPS_PROXY", "https://PROXY:8443")
		t.Setenv("NO_PROXY", "*.internal")

		cfg := ResolveProxy()
		assert.Equal(t, "http://PROXY:8080", cfg.HTTPProxy)
		assert.Equal(t, "https://PROXY:8443", cfg.HTTPSProxy)
		assert.Equal(t, "*.internal", cfg.NoProxy)
	})

	t.Run("lowercase takes precedence", func(t *testing.T) {
		t.Setenv("http_proxy", "http://lower:8080")
		t.Setenv("HTTP_PROXY", "http://UPPER:8080")

		cfg := ResolveProxy()
		assert.Equal(t, "http://lower:8080", cfg.HTTPProxy)
	})
}

func TestProxyConfig_HasProxy(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ProxyConfig
		expected bool
	}{
		{
			name:     "true when HTTP proxy set",
			cfg:      ProxyConfig{HTTPProxy: "http://proxy:8080"},
			expected: true,
		},
		{
			name:     "true when HTTPS proxy set",
			cfg:      ProxyConfig{HTTPSProxy: "https://proxy:8443"},
			expected: true,
		},
		{
			name:     "false when only NoProxy set",
			cfg:      ProxyConfig{NoProxy: "localhost"},
			expected: false,
		},
		{
			name:     "false when empty",
			cfg:      ProxyConfig{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.HasProxy())
		})
	}
}

func TestProxyConfig_ShouldBypassProxy(t *testing.T) {
	tests := []struct {
		name     string
		noProxy  string
		host     string
		expected bool
	}{
		{
			name:     "bypasses exact match",
			noProxy:  "localhost,example.com",
			host:     "example.com",
			expected: true,
		},
		{
			name:     "bypasses suffix match",
			noProxy:  ".internal.net",
			host:     "api.internal.net",
			expected: true,
		},
		{
			name:     "bypasses wildcard",
			noProxy:  "*",
			host:     "anything.com",
			expected: true,
		},
		{
			name:     "does not bypass non-matching host",
			noProxy:  "localhost,internal.net",
			host:     "external.com",
			expected: false,
		},
		{
			name:     "does not bypass when NoProxy empty",
			noProxy:  "",
			host:     "example.com",
			expected: false,
		},
		{
			name:     "handles whitespace in NoProxy",
			noProxy:  "localhost, example.com , internal.net",
			host:     "example.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ProxyConfig{NoProxy: tt.noProxy}
			assert.Equal(t, tt.expected, cfg.ShouldBypassProxy(tt.host))
		})
	}
}
