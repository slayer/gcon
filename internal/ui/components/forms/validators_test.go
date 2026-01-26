package forms

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "nil value",
			value:   nil,
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: true,
		},
		{
			name:    "whitespace string",
			value:   "   ",
			wantErr: true,
		},
		{
			name:    "non-empty string",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "empty string slice",
			value:   []string{},
			wantErr: true,
		},
		{
			name:    "non-empty string slice",
			value:   []string{"a", "b"},
			wantErr: false,
		},
		{
			name:    "zero int",
			value:   0,
			wantErr: false, // Numbers don't have "empty" concept for required
		},
		{
			name:    "non-zero int",
			value:   42,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequired(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGCPResourceName(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "empty string",
			value:   "",
			wantErr: false, // Let ValidateRequired handle empty
		},
		{
			name:    "valid single char",
			value:   "a",
			wantErr: false,
		},
		{
			name:    "valid simple name",
			value:   "my-instance",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			value:   "instance-123",
			wantErr: false,
		},
		{
			name:    "starts with number",
			value:   "123-instance",
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			value:   "My-Instance",
			wantErr: true,
		},
		{
			name:    "contains underscore",
			value:   "my_instance",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			value:   "my-instance-",
			wantErr: true,
		},
		{
			name:    "too long (64 chars)",
			value:   "a123456789012345678901234567890123456789012345678901234567890123",
			wantErr: true,
		},
		{
			name:    "exactly 63 chars",
			value:   "a12345678901234567890123456789012345678901234567890123456789012",
			wantErr: false,
		},
		{
			name:    "non-string value",
			value:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGCPResourceName(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateGCPLabelKey(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "valid simple",
			value:   "environment",
			wantErr: false,
		},
		{
			name:    "valid with hyphen",
			value:   "team-name",
			wantErr: false,
		},
		{
			name:    "valid with underscore",
			value:   "team_name",
			wantErr: false,
		},
		{
			name:    "starts with number",
			value:   "1team",
			wantErr: true,
		},
		{
			name:    "contains uppercase",
			value:   "Environment",
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGCPLabelKey(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNumber(t *testing.T) {
	tests := []struct {
		name    string
		min     int64
		max     int64
		value   any
		wantErr bool
	}{
		{
			name:    "within range",
			min:     0,
			max:     100,
			value:   50,
			wantErr: false,
		},
		{
			name:    "at min",
			min:     10,
			max:     100,
			value:   int64(10),
			wantErr: false,
		},
		{
			name:    "at max",
			min:     10,
			max:     100,
			value:   int64(100),
			wantErr: false,
		},
		{
			name:    "below min",
			min:     10,
			max:     100,
			value:   5,
			wantErr: true,
		},
		{
			name:    "above max",
			min:     10,
			max:     100,
			value:   150,
			wantErr: true,
		},
		{
			name:    "float64 value",
			min:     0,
			max:     100,
			value:   float64(50.5),
			wantErr: false,
		},
		{
			name:    "string value",
			min:     0,
			max:     100,
			value:   "50",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateNumber(tt.min, tt.max)
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStringLength(t *testing.T) {
	tests := []struct {
		name    string
		min     int
		max     int
		value   string
		wantErr bool
	}{
		{
			name:    "within range",
			min:     1,
			max:     10,
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "too short",
			min:     5,
			max:     10,
			value:   "hi",
			wantErr: true,
		},
		{
			name:    "too long",
			min:     1,
			max:     5,
			value:   "hello world",
			wantErr: true,
		},
		{
			name:    "exactly at min",
			min:     5,
			max:     10,
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "exactly at max",
			min:     1,
			max:     5,
			value:   "hello",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidateStringLength(tt.min, tt.max)
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		value    string
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "matches pattern",
			pattern: `^[a-z]+$`,
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "does not match",
			pattern: `^[a-z]+$`,
			value:   "Hello123",
			wantErr: true,
		},
		{
			name:     "custom error message",
			pattern:  `^[a-z]+$`,
			value:    "123",
			wantErr:  true,
			errorMsg: "must be lowercase letters only",
		},
		{
			name:    "empty string allowed",
			pattern: `^[a-z]+$`,
			value:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := ValidatePattern(tt.pattern, tt.errorMsg)
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid IP",
			value:   "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "valid all zeros",
			value:   "0.0.0.0",
			wantErr: false,
		},
		{
			name:    "valid max values",
			value:   "255.255.255.255",
			wantErr: false,
		},
		{
			name:    "octet too high",
			value:   "192.168.1.256",
			wantErr: true,
		},
		{
			name:    "invalid format",
			value:   "192.168.1",
			wantErr: true,
		},
		{
			name:    "hostname",
			value:   "localhost",
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: false,
		},
	}

	validator := ValidateIPAddress()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid CIDR",
			value:   "10.0.0.0/8",
			wantErr: false,
		},
		{
			name:    "valid /32",
			value:   "192.168.1.1/32",
			wantErr: false,
		},
		{
			name:    "valid /0",
			value:   "0.0.0.0/0",
			wantErr: false,
		},
		{
			name:    "prefix too high",
			value:   "10.0.0.0/33",
			wantErr: true,
		},
		{
			name:    "invalid IP",
			value:   "10.0.0.256/8",
			wantErr: true,
		},
		{
			name:    "missing prefix",
			value:   "10.0.0.0",
			wantErr: true,
		},
		{
			name:    "empty string",
			value:   "",
			wantErr: false,
		},
	}

	validator := ValidateCIDR()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateOneOf(t *testing.T) {
	validator := ValidateOneOf([]string{"small", "medium", "large"})

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid small", value: "small", wantErr: false},
		{name: "valid medium", value: "medium", wantErr: false},
		{name: "valid large", value: "large", wantErr: false},
		{name: "invalid xlarge", value: "xlarge", wantErr: true},
		{name: "empty string", value: "", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestComposeValidators(t *testing.T) {
	// Compose: required + min length + pattern
	validator := ComposeValidators(
		ValidateRequired,
		ValidateStringLength(3, 10),
		ValidatePattern(`^[a-z]+$`, "must be lowercase"),
	)

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{
			name:    "valid",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "fails required (empty)",
			value:   "",
			wantErr: true,
		},
		{
			name:    "fails length (too short)",
			value:   "hi",
			wantErr: true,
		},
		{
			name:    "fails pattern (uppercase)",
			value:   "Hello",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDiskSize(t *testing.T) {
	validator := ValidateDiskSize(10, 65536)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "valid size", value: 100, wantErr: false},
		{name: "at min", value: 10, wantErr: false},
		{name: "at max", value: int64(65536), wantErr: false},
		{name: "below min", value: 5, wantErr: true},
		{name: "above max", value: 70000, wantErr: true},
		{name: "float value", value: float64(100.5), wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
