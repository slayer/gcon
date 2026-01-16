package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetadataEditor(t *testing.T) {
	editor := NewMetadataEditor()

	assert.NotNil(t, editor.textarea)
	assert.Equal(t, "", editor.GetContent())
	assert.False(t, editor.IsDirty())
}

func TestMetadataEditor_SetContent(t *testing.T) {
	editor := NewMetadataEditor()
	content := "key=value"

	editor.SetContent(content)

	assert.Equal(t, content, editor.GetContent())
	assert.False(t, editor.IsDirty())
}

func TestMetadataEditor_IsDirty(t *testing.T) {
	editor := NewMetadataEditor()
	editor.SetContent("original")

	// Not dirty initially
	assert.False(t, editor.IsDirty())

	// Manually modify content (simulating user edit)
	editor.textarea.SetValue("modified")

	// Now it should be dirty
	assert.True(t, editor.IsDirty())
}

func TestMetadataEditor_SetSize(t *testing.T) {
	editor := NewMetadataEditor()

	editor.SetSize(80, 24)

	assert.Equal(t, 80, editor.width)
	assert.Equal(t, 24, editor.height)
}

func TestSerializeMetadata_Empty(t *testing.T) {
	metadata := map[string]string{}

	result := SerializeMetadata(metadata)

	assert.Equal(t, "", result)
}

func TestSerializeMetadata_SimpleValues(t *testing.T) {
	metadata := map[string]string{
		"environment": "production",
		"region":      "us-central1",
	}

	result := SerializeMetadata(metadata)

	// Order is not guaranteed, so check both are present
	assert.Contains(t, result, "environment=production")
	assert.Contains(t, result, "region=us-central1")
}

func TestSerializeMetadata_SSHKey(t *testing.T) {
	metadata := map[string]string{
		"ssh-keys": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC8... user@example.com",
	}

	result := SerializeMetadata(metadata)

	// SSH keys should use colon format for readability
	assert.Contains(t, result, "ssh-keys: ssh-rsa")
}

func TestSerializeMetadata_QuotedValues(t *testing.T) {
	metadata := map[string]string{
		"script": "#!/bin/bash\necho 'hello'\nexit 0",
	}

	result := SerializeMetadata(metadata)

	// Multi-line values should be quoted
	assert.Contains(t, result, "script=\"")
	assert.Contains(t, result, "\\n")
}

func TestSerializeMetadata_ValuesWithSpaces(t *testing.T) {
	metadata := map[string]string{
		"description": " value with leading space ",
	}

	result := SerializeMetadata(metadata)

	// Values with leading/trailing spaces should be quoted
	assert.Contains(t, result, "description=\"")
}

func TestParseMetadata_Empty(t *testing.T) {
	text := ""

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Empty(t, metadata)
}

func TestParseMetadata_SimpleKeyValue(t *testing.T) {
	text := "environment=production\nregion=us-central1"

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, "production", metadata["environment"])
	assert.Equal(t, "us-central1", metadata["region"])
}

func TestParseMetadata_ColonFormat(t *testing.T) {
	text := "environment: production\nregion: us-central1"

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, "production", metadata["environment"])
	assert.Equal(t, "us-central1", metadata["region"])
}

func TestParseMetadata_MixedFormats(t *testing.T) {
	text := "key1=value1\nkey2: value2"

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, "value2", metadata["key2"])
}

func TestParseMetadata_QuotedValues(t *testing.T) {
	text := `key1="value with spaces"
key2="value with \"quotes\""`

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, "value with spaces", metadata["key1"])
	assert.Equal(t, `value with "quotes"`, metadata["key2"])
}

func TestParseMetadata_MultilineIndented(t *testing.T) {
	text := `script: |
  #!/bin/bash
  echo "Hello"
  exit 0`

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	expected := "#!/bin/bash\necho \"Hello\"\nexit 0"
	assert.Equal(t, expected, metadata["script"])
}

func TestParseMetadata_SSHKey(t *testing.T) {
	text := "ssh-keys: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC8... user@example.com"

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Contains(t, metadata["ssh-keys"], "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC8...")
}

func TestParseMetadata_EmptyLines(t *testing.T) {
	text := `key1=value1

key2=value2

`

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, 2, len(metadata))
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, "value2", metadata["key2"])
}

func TestParseMetadata_Comments(t *testing.T) {
	text := `# This is a comment
key1=value1
# Another comment
key2=value2`

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, 2, len(metadata))
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, "value2", metadata["key2"])
}

func TestParseMetadata_InvalidFormat(t *testing.T) {
	text := "invalid line without separator"

	_, err := ParseMetadata(text)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestParseMetadata_Whitespace(t *testing.T) {
	text := "  key1  =  value1  \n  key2  :  value2  "

	metadata, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, "value1", metadata["key1"])
	assert.Equal(t, "value2", metadata["key2"])
}

func TestValidate_Empty(t *testing.T) {
	metadata := map[string]string{}

	errors := Validate(metadata)

	assert.Empty(t, errors)
}

func TestValidate_ValidMetadata(t *testing.T) {
	metadata := map[string]string{
		"environment": "production",
		"region":      "us-central1",
		"app_version": "1.0.0",
	}

	errors := Validate(metadata)

	assert.Empty(t, errors)
}

func TestValidate_EmptyKey(t *testing.T) {
	metadata := map[string]string{
		"": "value",
	}

	errors := Validate(metadata)

	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0], "Empty key name")
}

func TestValidate_KeyTooLong(t *testing.T) {
	longKey := strings.Repeat("a", 129)
	metadata := map[string]string{
		longKey: "value",
	}

	errors := Validate(metadata)

	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0], "too long")
}

func TestValidate_InvalidKeyCharacters(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"spaces", "key with spaces"},
		{"dots", "key.with.dots"},
		{"special", "key@special"},
		{"brackets", "key[0]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]string{
				tt.key: "value",
			}

			errors := Validate(metadata)

			assert.NotEmpty(t, errors)
			assert.Contains(t, errors[0], "invalid characters")
		})
	}
}

func TestValidate_ValidKeyCharacters(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"alphanumeric", "key123"},
		{"hyphens", "my-key"},
		{"underscores", "my_key"},
		{"mixed", "my_key-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := map[string]string{
				tt.key: "value",
			}

			errors := Validate(metadata)

			assert.Empty(t, errors, "key '%s' should be valid", tt.key)
		})
	}
}

func TestValidate_ValueTooLarge(t *testing.T) {
	largeValue := strings.Repeat("a", maxValueSize+1)
	metadata := map[string]string{
		"key": largeValue,
	}

	errors := Validate(metadata)

	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0], "too large")
}

func TestValidate_TooManyKeys(t *testing.T) {
	metadata := make(map[string]string)
	for i := 0; i < maxMetadataKeys+1; i++ {
		metadata[strings.Repeat("k", i+1)] = "value"
	}

	errors := Validate(metadata)

	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0], "Too many metadata keys")
}

func TestValidate_TotalSizeTooLarge(t *testing.T) {
	// Create metadata that exceeds total size limit
	metadata := make(map[string]string)
	// Each entry is about 10KB
	largeValue := strings.Repeat("a", 10000)
	for i := 0; i < 30; i++ {
		metadata[strings.Repeat("k", i+1)] = largeValue
	}

	errors := Validate(metadata)

	assert.NotEmpty(t, errors)
	assert.Contains(t, errors[0], "Total metadata size")
}

func TestValidate_ValidSSHKey(t *testing.T) {
	// Valid SSH keys
	validKeys := []string{
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC8eNgJL9vkJ+YL9ZxqN2v... user@example.com",
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFKhz7B3vZ9oJL3qN2v8L9ZxqN... user@host",
		"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTY...",
	}

	for _, key := range validKeys {
		metadata := map[string]string{
			"ssh-keys": key,
		}

		errors := Validate(metadata)

		assert.Empty(t, errors, "SSH key should be valid: %s", key)
	}
}

func TestValidate_InvalidSSHKey(t *testing.T) {
	// Invalid SSH keys
	invalidKeys := []string{
		"ssh-rsa short",                    // Too short
		"ssh-rsa INVALID@CHARS!!! comment", // Invalid base64
		"ssh-rsa ",                         // Missing key data (has space)
	}

	for _, key := range invalidKeys {
		metadata := map[string]string{
			"ssh-keys": key,
		}

		errors := Validate(metadata)

		assert.NotEmpty(t, errors, "SSH key should be invalid: %s", key)
		if len(errors) > 0 {
			assert.Contains(t, errors[0], "invalid format")
		}
	}
}

func TestIsSSHKey(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"ssh-rsa", "ssh-rsa AAAAB3...", true},
		{"ssh-ed25519", "ssh-ed25519 AAAAC3...", true},
		{"ecdsa-256", "ecdsa-sha2-nistp256 AAAAE2...", true},
		{"ecdsa-384", "ecdsa-sha2-nistp384 AAAAE2...", true},
		{"ecdsa-521", "ecdsa-sha2-nistp521 AAAAE2...", true},
		{"not ssh key", "just a regular value", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSSHKey(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateSSHKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{
			name:     "valid rsa with comment",
			key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC8eNgJL9vkJ+YL9ZxqN2v8L9ZxqN2v8L9ZxqN2v user@example.com",
			expected: true,
		},
		{
			name:     "valid ed25519",
			key:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFKhz7B3vZ9oJL3qN2v8L9ZxqN2v8L9ZxqN2v8L9ZxqN",
			expected: true,
		},
		{
			name:     "valid without comment",
			key:      "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC8eNgJL9vkJ+YL9ZxqN2v8L9ZxqN2v8L9ZxqN2v",
			expected: true,
		},
		{
			name:     "too short",
			key:      "ssh-rsa short",
			expected: false,
		},
		{
			name:     "invalid base64",
			key:      "ssh-rsa INVALID@CHARS!!!",
			expected: false,
		},
		{
			name:     "wrong algorithm",
			key:      "invalid-algo AAAAB3NzaC1yc2EAAAADAQABAAABgQC8eNgJL9vkJ+YL9ZxqN2v8L9ZxqN2v",
			expected: false,
		},
		{
			name:     "missing key data",
			key:      "ssh-rsa",
			expected: false,
		},
		{
			name:     "too many parts",
			key:      "ssh-rsa AAAAB3... comment extra parts",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateSSHKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedKey   string
		expectedValue string
		isMultiline   bool
		expectError   bool
	}{
		{
			name:          "equals format",
			line:          "key=value",
			expectedKey:   "key",
			expectedValue: "value",
			isMultiline:   false,
			expectError:   false,
		},
		{
			name:          "colon format",
			line:          "key: value",
			expectedKey:   "key",
			expectedValue: "value",
			isMultiline:   false,
			expectError:   false,
		},
		{
			name:          "quoted value equals",
			line:          `key="quoted value"`,
			expectedKey:   "key",
			expectedValue: "quoted value",
			isMultiline:   false,
			expectError:   false,
		},
		{
			name:          "quoted value colon",
			line:          `key: "quoted value"`,
			expectedKey:   "key",
			expectedValue: "quoted value",
			isMultiline:   false,
			expectError:   false,
		},
		{
			name:          "multiline indicator",
			line:          "key: |",
			expectedKey:   "key",
			expectedValue: "",
			isMultiline:   true,
			expectError:   false,
		},
		{
			name:        "invalid format",
			line:        "invalid line",
			expectError: true,
		},
		{
			name:          "whitespace trimmed",
			line:          "  key  =  value  ",
			expectedKey:   "key",
			expectedValue: "value",
			isMultiline:   false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, isMultiline, err := parseLine(tt.line)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedKey, key)
				assert.Equal(t, tt.expectedValue, value)
				assert.Equal(t, tt.isMultiline, isMultiline)
			}
		})
	}
}

func TestRoundTrip_SerializeAndParse(t *testing.T) {
	original := map[string]string{
		"environment": "production",
		"region":      "us-central1",
		"version":     "1.0.0",
	}

	// Serialize
	text := SerializeMetadata(original)

	// Parse back
	parsed, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}

func TestRoundTrip_WithSSHKeys(t *testing.T) {
	original := map[string]string{
		"ssh-keys":    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC8eNgJL9vkJ+YL9ZxqN2v8L9ZxqN2v8L9ZxqN2v user@example.com",
		"environment": "production",
	}

	// Serialize
	text := SerializeMetadata(original)

	// Parse back
	parsed, err := ParseMetadata(text)

	require.NoError(t, err)
	assert.Equal(t, original, parsed)
}
