package gcp

import (
	"strings"
)

// InstanceMetadata represents VM instance metadata with fingerprint for optimistic locking
type InstanceMetadata struct {
	Items       map[string]string
	Fingerprint string // Used for optimistic locking when updating metadata
}

// SSHKey represents a parsed SSH public key from instance metadata
type SSHKey struct {
	Username string // Linux username for this key
	KeyType  string // e.g., "ssh-rsa", "ssh-ed25519"
	KeyData  string // The actual public key data
	Comment  string // Optional comment (usually user@host)
}

// ParseSSHKeys parses SSH keys from the ssh-keys metadata value
// Format: "username:ssh-rsa KEY user@host\nusername2:ssh-ed25519 KEY2 user2@host2"
// Each key is on a new line
func ParseSSHKeys(sshKeysValue string) []SSHKey {
	if sshKeysValue == "" {
		return nil
	}

	var keys []SSHKey
	lines := strings.Split(sshKeysValue, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split on first colon to separate username from key
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		username := strings.TrimSpace(parts[0])
		keyPart := strings.TrimSpace(parts[1])

		// Parse key part: "ssh-rsa KEY comment" or "ssh-ed25519 KEY comment"
		keyFields := strings.Fields(keyPart)
		if len(keyFields) < 2 {
			continue
		}

		key := SSHKey{
			Username: username,
			KeyType:  keyFields[0],
			KeyData:  keyFields[1],
		}

		// Remaining fields are the comment
		if len(keyFields) > 2 {
			key.Comment = strings.Join(keyFields[2:], " ")
		}

		keys = append(keys, key)
	}

	return keys
}

// FormatSSHKeys converts SSH keys back to the metadata format
// Returns a string suitable for the ssh-keys metadata value
func FormatSSHKeys(keys []SSHKey) string {
	if len(keys) == 0 {
		return ""
	}

	var lines []string
	for _, key := range keys {
		line := key.Username + ":" + key.KeyType + " " + key.KeyData
		if key.Comment != "" {
			line += " " + key.Comment
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
