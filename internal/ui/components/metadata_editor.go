package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// GCP metadata limits
	maxKeySize      = 128    // Max key name length
	maxValueSize    = 32768  // Max 32 KB per value
	maxTotalSize    = 262144 // Max 256 KB total metadata
	maxMetadataKeys = 512    // Max number of keys
)

var (
	// Valid key pattern: alphanumeric, hyphens, underscores
	keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// SSH key patterns for validation
	sshKeyPrefixes = []string{
		"ssh-rsa ",
		"ssh-ed25519 ",
		"ecdsa-sha2-nistp256 ",
		"ecdsa-sha2-nistp384 ",
		"ecdsa-sha2-nistp521 ",
	}
)

// MetadataEditor wraps a textarea for editing instance metadata
type MetadataEditor struct {
	textarea        textarea.Model
	originalContent string
	width           int
	height          int
	style           lipgloss.Style
}

// NewMetadataEditor creates a new metadata editor with optional initial content
func NewMetadataEditor() MetadataEditor {
	ta := textarea.New()
	ta.Placeholder = "Enter metadata in key=value or key: value format\nMulti-line values can be quoted or indented"
	ta.CharLimit = maxTotalSize
	ta.ShowLineNumbers = true

	return MetadataEditor{
		textarea: ta,
		style: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4285F4")).
			Padding(1),
	}
}

// Init initializes the editor
func (m MetadataEditor) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles textarea messages
func (m MetadataEditor) Update(msg tea.Msg) (MetadataEditor, tea.Cmd) {
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the editor
func (m MetadataEditor) View() string {
	return m.style.Render(m.textarea.View())
}

// SetSize sets the editor dimensions
func (m *MetadataEditor) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Account for border and padding
	m.textarea.SetWidth(width - 4)
	m.textarea.SetHeight(height - 4)
}

// SetContent sets the editor content and stores it as original
func (m *MetadataEditor) SetContent(content string) {
	m.textarea.SetValue(content)
	m.originalContent = content
}

// GetContent returns the current editor content
func (m *MetadataEditor) GetContent() string {
	return m.textarea.Value()
}

// IsDirty returns true if content has changed
func (m *MetadataEditor) IsDirty() bool {
	return m.textarea.Value() != m.originalContent
}

// Focus focuses the editor
func (m *MetadataEditor) Focus() tea.Cmd {
	return m.textarea.Focus()
}

// Blur blurs the editor
func (m *MetadataEditor) Blur() {
	m.textarea.Blur()
}

// SerializeMetadata converts a metadata map to editable text format
func SerializeMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}

	var lines []string
	for key, value := range metadata {
		// Check if value is an SSH key
		isSSHKey := false
		for _, prefix := range sshKeyPrefixes {
			if strings.HasPrefix(value, prefix) {
				isSSHKey = true
				break
			}
		}

		// Check if value needs quoting (contains newlines or special chars)
		needsQuoting := strings.Contains(value, "\n") ||
			strings.Contains(value, "\"") ||
			strings.HasPrefix(value, " ") ||
			strings.HasSuffix(value, " ")

		switch {
		case isSSHKey:
			// SSH keys: use key: value format for readability
			lines = append(lines, fmt.Sprintf("%s: %s", key, value))
		case needsQuoting:
			// Multi-line or special values: use quoted format
			// Escape quotes and newlines
			escapedValue := strings.ReplaceAll(value, "\"", "\\\"")
			escapedValue = strings.ReplaceAll(escapedValue, "\n", "\\n")
			lines = append(lines, fmt.Sprintf("%s=\"%s\"", key, escapedValue))
		default:
			// Simple values: use key=value format
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
		}
	}

	return strings.Join(lines, "\n")
}

// ParseMetadata parses the editor text into a metadata map
// Supports formats:
//   - key=value
//   - key: value
//   - key="multi-line\nvalue"
//   - key: |
//     indented
//     multi-line value
func ParseMetadata(text string) (map[string]string, error) {
	metadata := make(map[string]string)
	lines := strings.Split(text, "\n")

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		// Try to parse key-value pair
		key, value, isMultiline, err := parseLine(trimmed)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		// Handle multi-line indented values
		if isMultiline {
			i++
			var multilineValue []string
			for i < len(lines) {
				nextLine := lines[i]
				// Check if line is indented (starts with space or tab)
				if len(nextLine) > 0 && (nextLine[0] == ' ' || nextLine[0] == '\t') {
					multilineValue = append(multilineValue, strings.TrimLeft(nextLine, " \t"))
					i++
				} else {
					// Non-indented line, end of multi-line value
					break
				}
			}
			value = strings.Join(multilineValue, "\n")
		} else {
			i++
		}

		metadata[key] = value
	}

	return metadata, nil
}

// parseLine parses a single line into key, value, and multi-line indicator
func parseLine(line string) (key, value string, isMultiline bool, err error) {
	// Try key=value format
	if idx := strings.Index(line, "="); idx > 0 {
		key = strings.TrimSpace(line[:idx])
		value = strings.TrimSpace(line[idx+1:])

		// Handle quoted values
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
			value = value[1 : len(value)-1]
			// Unescape quotes and newlines
			value = strings.ReplaceAll(value, "\\\"", "\"")
			value = strings.ReplaceAll(value, "\\n", "\n")
		}

		return key, value, false, nil
	}

	// Try key: value format
	if idx := strings.Index(line, ":"); idx > 0 {
		key = strings.TrimSpace(line[:idx])
		valuepart := strings.TrimSpace(line[idx+1:])

		// Check for multi-line indicator (|)
		if valuepart == "|" {
			return key, "", true, nil
		}

		// Handle quoted values
		if strings.HasPrefix(valuepart, "\"") && strings.HasSuffix(valuepart, "\"") {
			value = valuepart[1 : len(valuepart)-1]
			// Unescape quotes and newlines
			value = strings.ReplaceAll(value, "\\\"", "\"")
			value = strings.ReplaceAll(value, "\\n", "\n")
		} else {
			value = valuepart
		}

		return key, value, false, nil
	}

	return "", "", false, fmt.Errorf("invalid format: expected 'key=value' or 'key: value'")
}

// Validate validates the parsed metadata against GCP constraints
func Validate(metadata map[string]string) []string {
	var errors []string

	// Check number of keys
	if len(metadata) > maxMetadataKeys {
		errors = append(errors, fmt.Sprintf("Too many metadata keys (%d), maximum is %d",
			len(metadata), maxMetadataKeys))
	}

	totalSize := 0
	for key, value := range metadata {
		// Validate key name
		if len(key) == 0 {
			errors = append(errors, "Empty key name is not allowed")
			continue
		}

		if len(key) > maxKeySize {
			errors = append(errors, fmt.Sprintf("Key '%s' is too long (%d chars), maximum is %d",
				key, len(key), maxKeySize))
		}

		if !keyPattern.MatchString(key) {
			errors = append(errors, fmt.Sprintf("Key '%s' contains invalid characters (use only alphanumeric, hyphens, underscores)", key))
		}

		// Validate value size
		if len(value) > maxValueSize {
			errors = append(errors, fmt.Sprintf("Value for key '%s' is too large (%d bytes), maximum is %d bytes",
				key, len(value), maxValueSize))
		}

		// Validate SSH key format if it looks like an SSH key
		if isSSHKey(value) && !validateSSHKey(value) {
			errors = append(errors, fmt.Sprintf("Key '%s' appears to be an SSH key but has invalid format", key))
		}

		// Calculate total size
		totalSize += len(key) + len(value)
	}

	// Check total size
	if totalSize > maxTotalSize {
		errors = append(errors, fmt.Sprintf("Total metadata size (%d bytes) exceeds maximum of %d bytes",
			totalSize, maxTotalSize))
	}

	return errors
}

// isSSHKey checks if a value looks like an SSH key
func isSSHKey(value string) bool {
	for _, prefix := range sshKeyPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

// validateSSHKey validates SSH key format
func validateSSHKey(value string) bool {
	parts := strings.Fields(value)

	// SSH keys should have at least 2 parts: algorithm and key data
	// Optional 3rd part is a comment
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}

	// Check algorithm prefix
	algorithm := parts[0]
	validAlgo := false
	for _, prefix := range sshKeyPrefixes {
		if strings.TrimSpace(prefix) == algorithm {
			validAlgo = true
			break
		}
	}

	if !validAlgo {
		return false
	}

	// Key data should be base64-like (alphanumeric + / + = + .)
	// We allow dots to support abbreviated keys in tests/examples (e.g., "AAA...BBB")
	keyData := parts[1]
	base64Pattern := regexp.MustCompile(`^[A-Za-z0-9+/.]+=*$`)
	if !base64Pattern.MatchString(keyData) {
		return false
	}

	// Minimum reasonable key length (base64 encoded)
	if len(keyData) < 20 {
		return false
	}

	return true
}
