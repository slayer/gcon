package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ServiceAccountCredentials represents the structure of a GCP service account JSON key file
type ServiceAccountCredentials struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
}

// GetAuthenticatedIdentity returns the email/identity of the currently
// authenticated user or service account. Returns empty string if unable to determine.
//
// Tries two methods in order:
// 1. User credentials from gcloud config (gcloud auth application-default login)
// 2. Service account from GOOGLE_APPLICATION_CREDENTIALS JSON file
func GetAuthenticatedIdentity() (string, error) {
	// Method 1: Try gcloud config (user credentials)
	if account := ResolveAccount(""); account != "" {
		return account, nil
	}

	// Method 2: Try service account credentials file
	if credFile := ResolveCredentialsFile(); credFile != "" {
		email, err := extractServiceAccountEmail(credFile)
		if err == nil && email != "" {
			return email, nil
		}
		// Log error but continue - not critical
		if err != nil {
			return "", fmt.Errorf("failed to extract service account email: %w", err)
		}
	}

	return "", fmt.Errorf("unable to determine authenticated identity")
}

// extractServiceAccountEmail reads a service account credentials JSON file
// and extracts the client_email field.
func extractServiceAccountEmail(credentialsPath string) (string, error) {
	// Read the credentials file
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse JSON
	var creds ServiceAccountCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("failed to parse credentials JSON: %w", err)
	}

	// Extract client_email
	if creds.ClientEmail == "" {
		return "", fmt.Errorf("client_email field is empty or missing")
	}

	return creds.ClientEmail, nil
}
