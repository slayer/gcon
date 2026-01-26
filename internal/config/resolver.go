package config

import (
	"os"
	"strings"
)

// ResolveProject determines which project to use based on priority:
// 1. CLI flag (passed as argument)
// 2. CLOUDSDK_CORE_PROJECT env var
// 3. gcloud config active configuration
// Returns empty string if no default found (should show selector)
func ResolveProject(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if envProject := os.Getenv("CLOUDSDK_CORE_PROJECT"); envProject != "" {
		return envProject
	}

	config, err := LoadGcloudConfig()
	if err != nil {
		return ""
	}

	return config.Project
}

// ResolveZone determines which zone to use based on priority:
// 1. CLI flag (passed as argument)
// 2. CLOUDSDK_COMPUTE_ZONE env var
// 3. gcloud config compute/zone
func ResolveZone(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if envZone := os.Getenv("CLOUDSDK_COMPUTE_ZONE"); envZone != "" {
		return envZone
	}

	config, err := LoadGcloudConfig()
	if err != nil {
		return ""
	}

	return config.Zone
}

// ResolveRegion determines which region to use based on priority:
// 1. CLI flag (passed as argument)
// 2. CLOUDSDK_COMPUTE_REGION env var
// 3. gcloud config compute/region
func ResolveRegion(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if envRegion := os.Getenv("CLOUDSDK_COMPUTE_REGION"); envRegion != "" {
		return envRegion
	}

	config, err := LoadGcloudConfig()
	if err != nil {
		return ""
	}

	return config.Region
}

// ResolveAccount determines which account to use based on priority:
// 1. CLI flag (passed as argument)
// 2. CLOUDSDK_CORE_ACCOUNT env var
// 3. gcloud config core/account
func ResolveAccount(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	if envAccount := os.Getenv("CLOUDSDK_CORE_ACCOUNT"); envAccount != "" {
		return envAccount
	}

	config, err := LoadGcloudConfig()
	if err != nil {
		return ""
	}

	return config.Account
}

// ResolveActiveConfigName returns the gcloud configuration name to use.
// Priority: CLOUDSDK_ACTIVE_CONFIG_NAME env var → gcloud properties file → "default"
func ResolveActiveConfigName() string {
	if envConfig := os.Getenv("CLOUDSDK_ACTIVE_CONFIG_NAME"); envConfig != "" {
		return envConfig
	}

	// Fall back to reading from gcloud config
	config, err := LoadGcloudConfig()
	if err != nil || config == nil {
		return "default"
	}

	if config.ActiveConfig != "" {
		return config.ActiveConfig
	}

	return "default"
}

// ResolveCredentialsFile returns the path to service account credentials JSON.
// This is read-only from GOOGLE_APPLICATION_CREDENTIALS env var.
func ResolveCredentialsFile() string {
	return os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
}

// NoColor returns true if color output should be disabled.
// Respects the NO_COLOR standard (https://no-color.org/)
func NoColor() bool {
	_, exists := os.LookupEnv("NO_COLOR")
	return exists
}

// ProxyConfig holds HTTP/HTTPS proxy settings
type ProxyConfig struct {
	HTTPProxy  string
	HTTPSProxy string
	NoProxy    string
}

// ResolveProxy returns proxy configuration from environment.
// Checks both lowercase and uppercase variants (lowercase takes precedence per convention).
func ResolveProxy() ProxyConfig {
	return ProxyConfig{
		HTTPProxy:  getEnvWithFallback("http_proxy", "HTTP_PROXY"),
		HTTPSProxy: getEnvWithFallback("https_proxy", "HTTPS_PROXY"),
		NoProxy:    getEnvWithFallback("no_proxy", "NO_PROXY"),
	}
}

// HasProxy returns true if any proxy is configured
func (p ProxyConfig) HasProxy() bool {
	return p.HTTPProxy != "" || p.HTTPSProxy != ""
}

// ShouldBypassProxy checks if the given host should bypass the proxy
func (p ProxyConfig) ShouldBypassProxy(host string) bool {
	if p.NoProxy == "" {
		return false
	}
	for _, bypass := range strings.Split(p.NoProxy, ",") {
		bypass = strings.TrimSpace(bypass)
		if bypass == "*" || bypass == host || strings.HasSuffix(host, bypass) {
			return true
		}
	}
	return false
}

func getEnvWithFallback(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}
