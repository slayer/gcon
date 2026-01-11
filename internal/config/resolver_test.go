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
