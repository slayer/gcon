package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	// ErrIdentityNotDetermined is returned when unable to determine authenticated identity
	ErrIdentityNotDetermined = errors.New("unable to determine authenticated identity")
	// ErrClientEmailMissing is returned when service account JSON is missing client_email
	ErrClientEmailMissing = errors.New("client_email field is empty or missing")
)

// IdentityType represents the type of authenticated identity
type IdentityType int

const (
	IdentityUnknown IdentityType = iota
	IdentityUser
	IdentityServiceAccount
)

// String returns the string representation of IdentityType
func (t IdentityType) String() string {
	switch t {
	case IdentityUser:
		return "user"
	case IdentityServiceAccount:
		return "service_account"
	default:
		return "unknown"
	}
}

// ServiceAccountCredentials represents the structure of a GCP service account JSON key file
type ServiceAccountCredentials struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
}

// identityCache stores the cached identity to avoid repeated file reads
var (
	cachedIdentity     string
	cachedIdentityType IdentityType
	cacheInitialized   bool
	cacheMutex         sync.RWMutex
)

// GetAuthenticatedIdentity returns the email/identity and type of the currently
// authenticated user or service account. Returns empty string if unable to determine.
// Results are cached to avoid repeated file reads.
//
// Tries two methods in order:
// 1. User credentials from gcloud config (gcloud auth application-default login)
// 2. Service account from GOOGLE_APPLICATION_CREDENTIALS JSON file
func GetAuthenticatedIdentity() (string, IdentityType, error) {
	// Check cache first
	cacheMutex.RLock()
	if cacheInitialized {
		identity := cachedIdentity
		identityType := cachedIdentityType
		cacheMutex.RUnlock()
		if identity != "" {
			return identity, identityType, nil
		}
		return "", IdentityUnknown, ErrIdentityNotDetermined
	}
	cacheMutex.RUnlock()

	// Acquire write lock to populate cache
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if cacheInitialized {
		if cachedIdentity != "" {
			return cachedIdentity, cachedIdentityType, nil
		}
		return "", IdentityUnknown, ErrIdentityNotDetermined
	}

	// Method 1: Try gcloud config (user credentials)
	if account := ResolveAccount(""); account != "" {
		cachedIdentity = account
		cachedIdentityType = IdentityUser
		cacheInitialized = true
		return account, IdentityUser, nil
	}

	// Method 2: Try service account credentials file
	if credFile := ResolveCredentialsFile(); credFile != "" {
		email, err := extractServiceAccountEmail(credFile)
		if err == nil && email != "" {
			cachedIdentity = email
			cachedIdentityType = IdentityServiceAccount
			cacheInitialized = true
			return email, IdentityServiceAccount, nil
		}
		// Mark cache as initialized even on error to avoid repeated attempts
		cacheInitialized = true
		if err != nil {
			return "", IdentityUnknown, fmt.Errorf("failed to extract service account email: %w", err)
		}
	}

	// Mark cache as initialized with no identity found
	cacheInitialized = true
	return "", IdentityUnknown, ErrIdentityNotDetermined
}

// ClearIdentityCache clears the cached identity (useful for testing)
func ClearIdentityCache() {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	cachedIdentity = ""
	cachedIdentityType = IdentityUnknown
	cacheInitialized = false
}

// extractServiceAccountEmail reads a service account credentials JSON file
// and extracts the client_email field.
func extractServiceAccountEmail(credentialsPath string) (string, error) {
	// Read the credentials file
	data, err := os.ReadFile(credentialsPath) // #nosec G304 -- Reading user-provided credentials path
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
		return "", ErrClientEmailMissing
	}

	return creds.ClientEmail, nil
}
