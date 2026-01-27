package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrNoGcloudConfig is returned when gcloud is not configured
var ErrNoGcloudConfig = errors.New("gcloud not configured")

// GcloudConfig holds parsed gcloud configuration
type GcloudConfig struct {
	ActiveConfig string
	Project      string
	Account      string
	Zone         string
	Region       string
}

// LoadGcloudConfig reads the default project from gcloud configuration.
// Returns ErrNoGcloudConfig if gcloud is not configured or no project is set.
func LoadGcloudConfig() (*GcloudConfig, error) {
	configDir := getConfigDir()

	// Check if gcloud config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, ErrNoGcloudConfig
	}

	// Determine active configuration name
	// Priority: CLOUDSDK_ACTIVE_CONFIG_NAME env → properties file → "default"
	activeConfig := os.Getenv("CLOUDSDK_ACTIVE_CONFIG_NAME")
	if activeConfig == "" {
		var err error
		activeConfig, err = getActiveConfig(configDir)
		if err != nil {
			return nil, err
		}
	}
	if activeConfig == "" {
		activeConfig = "default"
	}

	// Parse the active configuration file
	configPath := filepath.Join(configDir, "configurations", "config_"+activeConfig)
	configData, err := parseConfigFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoGcloudConfig
		}
		return nil, err
	}

	config := &GcloudConfig{
		ActiveConfig: activeConfig,
	}

	if core, ok := configData["core"]; ok {
		config.Project = core["project"]
		config.Account = core["account"]
	}

	// Zone and region are in [compute] section
	if compute, ok := configData["compute"]; ok {
		config.Zone = compute["zone"]
		config.Region = compute["region"]
	}

	return config, nil
}

// getConfigDir returns gcloud config directory.
// Respects CLOUDSDK_CONFIG env var, falls back to ~/.config/gcloud
func getConfigDir() string {
	if dir := os.Getenv("CLOUDSDK_CONFIG"); dir != "" {
		return dir
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".config", "gcloud")
}

// getActiveConfig reads which configuration is active.
// Gcloud stores the active config name in the 'active_config' file.
func getActiveConfig(configDir string) (string, error) {
	activeConfigPath := filepath.Join(configDir, "active_config")

	// Read the active_config file (contains just the config name)
	data, err := os.ReadFile(activeConfigPath) // #nosec G304 -- Reading gcloud config file
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	// The file contains just the config name, possibly with trailing newline
	configName := strings.TrimSpace(string(data))
	return configName, nil
}

// parseConfigFile parses an INI-style gcloud config file.
// Returns map[section]map[key]value
func parseConfigFile(path string) (map[string]map[string]string, error) {
	file, err := os.Open(path) // #nosec G304 -- Reading gcloud config file
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // Best-effort cleanup

	result := make(map[string]map[string]string)
	var currentSection string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header [section_name]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimPrefix(strings.TrimSuffix(line, "]"), "[")
			if result[currentSection] == nil {
				result[currentSection] = make(map[string]string)
			}
			continue
		}

		// Key-value pair
		if currentSection != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				result[currentSection][key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
