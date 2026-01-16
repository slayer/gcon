package gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSSHKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []SSHKey
	}{
		{
			name:  "single ssh-rsa key",
			input: "alice:ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC alice@example.com",
			expected: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2EAAAADAQABAAABgQC",
					Comment:  "alice@example.com",
				},
			},
		},
		{
			name:  "single ssh-ed25519 key",
			input: "bob:ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILZm bob@host",
			expected: []SSHKey{
				{
					Username: "bob",
					KeyType:  "ssh-ed25519",
					KeyData:  "AAAAC3NzaC1lZDI1NTE5AAAAILZm",
					Comment:  "bob@host",
				},
			},
		},
		{
			name:  "multiple keys",
			input: "alice:ssh-rsa AAAAB3NzaC1yc2EAAA alice@example.com\nbob:ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 bob@host",
			expected: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2EAAA",
					Comment:  "alice@example.com",
				},
				{
					Username: "bob",
					KeyType:  "ssh-ed25519",
					KeyData:  "AAAAC3NzaC1lZDI1NTE5",
					Comment:  "bob@host",
				},
			},
		},
		{
			name:  "key without comment",
			input: "charlie:ssh-rsa AAAAB3NzaC1yc2EAAADAQ",
			expected: []SSHKey{
				{
					Username: "charlie",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2EAAADAQ",
					Comment:  "",
				},
			},
		},
		{
			name:  "key with extra whitespace",
			input: "  dave  :  ssh-rsa   AAAAB3NzaC1yc2E   dave@server  ",
			expected: []SSHKey{
				{
					Username: "dave",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2E",
					Comment:  "dave@server",
				},
			},
		},
		{
			name:  "empty lines and whitespace",
			input: "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host\n\n  \nbob:ssh-ed25519 AAAAC3NzaC1lZDI1 bob@host",
			expected: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2E",
					Comment:  "alice@host",
				},
				{
					Username: "bob",
					KeyType:  "ssh-ed25519",
					KeyData:  "AAAAC3NzaC1lZDI1",
					Comment:  "bob@host",
				},
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only whitespace",
			input:    "   \n  \n  ",
			expected: nil,
		},
		{
			name:     "malformed line without colon",
			input:    "alice ssh-rsa AAAAB3NzaC1yc2E",
			expected: nil,
		},
		{
			name:     "malformed line with only username",
			input:    "alice:",
			expected: nil,
		},
		{
			name:     "malformed line with only key type",
			input:    "alice:ssh-rsa",
			expected: nil,
		},
		{
			name:  "mixed valid and invalid keys",
			input: "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host\ninvalid-line\nbob:ssh-ed25519 AAAAC3NzaC1lZDI1 bob@host",
			expected: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2E",
					Comment:  "alice@host",
				},
				{
					Username: "bob",
					KeyType:  "ssh-ed25519",
					KeyData:  "AAAAC3NzaC1lZDI1",
					Comment:  "bob@host",
				},
			},
		},
		{
			name:  "comment with spaces",
			input: "alice:ssh-rsa AAAAB3NzaC1yc2E Alice Smith - Workstation",
			expected: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2E",
					Comment:  "Alice Smith - Workstation",
				},
			},
		},
		{
			name:  "ecdsa key",
			input: "eve:ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY eve@laptop",
			expected: []SSHKey{
				{
					Username: "eve",
					KeyType:  "ecdsa-sha2-nistp256",
					KeyData:  "AAAAE2VjZHNhLXNoYTItbmlzdHAyNTY",
					Comment:  "eve@laptop",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseSSHKeys(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatSSHKeys(t *testing.T) {
	tests := []struct {
		name     string
		keys     []SSHKey
		expected string
	}{
		{
			name: "single key with comment",
			keys: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2EAAAADAQABAAABgQC",
					Comment:  "alice@example.com",
				},
			},
			expected: "alice:ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC alice@example.com",
		},
		{
			name: "single key without comment",
			keys: []SSHKey{
				{
					Username: "bob",
					KeyType:  "ssh-ed25519",
					KeyData:  "AAAAC3NzaC1lZDI1NTE5AAAAILZm",
					Comment:  "",
				},
			},
			expected: "bob:ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAILZm",
		},
		{
			name: "multiple keys",
			keys: []SSHKey{
				{
					Username: "alice",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2EAAA",
					Comment:  "alice@example.com",
				},
				{
					Username: "bob",
					KeyType:  "ssh-ed25519",
					KeyData:  "AAAAC3NzaC1lZDI1NTE5",
					Comment:  "bob@host",
				},
			},
			expected: "alice:ssh-rsa AAAAB3NzaC1yc2EAAA alice@example.com\nbob:ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 bob@host",
		},
		{
			name:     "empty slice",
			keys:     []SSHKey{},
			expected: "",
		},
		{
			name:     "nil slice",
			keys:     nil,
			expected: "",
		},
		{
			name: "key with multi-word comment",
			keys: []SSHKey{
				{
					Username: "charlie",
					KeyType:  "ssh-rsa",
					KeyData:  "AAAAB3NzaC1yc2E",
					Comment:  "Charlie Brown - Main Laptop",
				},
			},
			expected: "charlie:ssh-rsa AAAAB3NzaC1yc2E Charlie Brown - Main Laptop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSSHKeys(tt.keys)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAndFormatRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // Expected output after round-trip (may differ due to normalization)
	}{
		{
			name:     "single key",
			input:    "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host",
			expected: "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host",
		},
		{
			name:     "multiple keys",
			input:    "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host\nbob:ssh-ed25519 AAAAC3NzaC1lZDI1 bob@host",
			expected: "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host\nbob:ssh-ed25519 AAAAC3NzaC1lZDI1 bob@host",
		},
		{
			name:     "key with extra whitespace (normalized)",
			input:    "  alice  :  ssh-rsa   AAAAB3NzaC1yc2E   alice@host  ",
			expected: "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host",
		},
		{
			name:     "keys with empty lines (normalized)",
			input:    "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host\n\n  \nbob:ssh-ed25519 AAAAC3NzaC1lZDI1 bob@host",
			expected: "alice:ssh-rsa AAAAB3NzaC1yc2E alice@host\nbob:ssh-ed25519 AAAAC3NzaC1lZDI1 bob@host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := ParseSSHKeys(tt.input)
			result := FormatSSHKeys(keys)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstanceMetadataStruct(t *testing.T) {
	t.Run("create empty metadata", func(t *testing.T) {
		metadata := &InstanceMetadata{
			Items:       make(map[string]string),
			Fingerprint: "abc123",
		}

		assert.NotNil(t, metadata.Items)
		assert.Equal(t, "abc123", metadata.Fingerprint)
		assert.Empty(t, metadata.Items)
	})

	t.Run("create metadata with items", func(t *testing.T) {
		metadata := &InstanceMetadata{
			Items: map[string]string{
				"startup-script": "#!/bin/bash\necho hello",
				"enable-oslogin": "TRUE",
			},
			Fingerprint: "xyz789",
		}

		assert.Len(t, metadata.Items, 2)
		assert.Equal(t, "#!/bin/bash\necho hello", metadata.Items["startup-script"])
		assert.Equal(t, "TRUE", metadata.Items["enable-oslogin"])
		assert.Equal(t, "xyz789", metadata.Fingerprint)
	})
}

func TestSSHKeyStruct(t *testing.T) {
	t.Run("create ssh key", func(t *testing.T) {
		key := SSHKey{
			Username: "alice",
			KeyType:  "ssh-rsa",
			KeyData:  "AAAAB3NzaC1yc2E",
			Comment:  "alice@example.com",
		}

		assert.Equal(t, "alice", key.Username)
		assert.Equal(t, "ssh-rsa", key.KeyType)
		assert.Equal(t, "AAAAB3NzaC1yc2E", key.KeyData)
		assert.Equal(t, "alice@example.com", key.Comment)
	})
}

func TestRealWorldSSHKeysExample(t *testing.T) {
	// Test with a realistic example from GCP documentation
	input := `alice:ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCzCFk+L5KNlpZrNFo9w8gVoTTl93LhZe8LQcMoINWQxL3SZ/mqZcFv alice.smith@company.com
bob:ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl bob@workstation
charlie:ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTY charlie-laptop`

	keys := ParseSSHKeys(input)
	assert.Len(t, keys, 3)

	// Check first key
	assert.Equal(t, "alice", keys[0].Username)
	assert.Equal(t, "ssh-rsa", keys[0].KeyType)
	assert.Equal(t, "alice.smith@company.com", keys[0].Comment)

	// Check second key
	assert.Equal(t, "bob", keys[1].Username)
	assert.Equal(t, "ssh-ed25519", keys[1].KeyType)
	assert.Equal(t, "bob@workstation", keys[1].Comment)

	// Check third key
	assert.Equal(t, "charlie", keys[2].Username)
	assert.Equal(t, "ecdsa-sha2-nistp256", keys[2].KeyType)
	assert.Equal(t, "charlie-laptop", keys[2].Comment)

	// Round-trip test
	formatted := FormatSSHKeys(keys)
	reparsed := ParseSSHKeys(formatted)
	assert.Equal(t, keys, reparsed)
}
