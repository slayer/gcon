package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// GcloudConfig holds parsed gcloud configuration
type GcloudConfig struct {
	ActiveConfig string
	Project      string
	Account      string
}

// LoadGcloudConfig reads the default project from gcloud configuration.
// Returns nil if gcloud is not configured or no project is set.
func LoadGcloudConfig() (*GcloudConfig, error) {
	configDir := getConfigDir()

	// Check if gcloud config directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, nil
	}

	// Determine active configuration name
	activeConfig, err := getActiveConfig(configDir)
	if err != nil {
		return nil, err
	}
	if activeConfig == "" {
		activeConfig = "default"
	}

	// Parse the active configuration file
	configPath := filepath.Join(configDir, "configurations", "config_"+activeConfig)
	configData, err := parseConfigFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
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

// getActiveConfig reads which configuration is active from properties file.
// The properties file contains [core] section with config = <name>
func getActiveConfig(configDir string) (string, error) {
	propertiesPath := filepath.Join(configDir, "properties")

	data, err := parseConfigFile(propertiesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	if core, ok := data["core"]; ok {
		return core["config"], nil
	}

	return "", nil
}

// parseConfigFile parses an INI-style gcloud config file.
// Returns map[section]map[key]value
func parseConfigFile(path string) (map[string]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

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
