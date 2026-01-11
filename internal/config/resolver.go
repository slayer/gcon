package config

import "os"

// ResolveProject determines which project to use based on priority:
// 1. CLI flag (passed as argument)
// 2. CLOUDSDK_CORE_PROJECT env var
// 3. gcloud config active configuration
// Returns empty string if no default found (should show selector)
func ResolveProject(flagValue string) string {
	// Priority 1: CLI flag
	if flagValue != "" {
		return flagValue
	}

	// Priority 2: Environment variable
	if envProject := os.Getenv("CLOUDSDK_CORE_PROJECT"); envProject != "" {
		return envProject
	}

	// Priority 3: gcloud config
	config, err := LoadGcloudConfig()
	if err != nil || config == nil {
		return ""
	}

	return config.Project
}
