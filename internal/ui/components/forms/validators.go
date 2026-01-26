package forms

import (
	"fmt"
	"regexp"
	"strings"
)

// GCP resource name pattern: lowercase letters, numbers, hyphens
// Must start with a lowercase letter, 1-63 characters
var gcpResourceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)

// GCP label key pattern: lowercase letters, numbers, hyphens, underscores
// Must start with a lowercase letter
var gcpLabelKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// GCP label value pattern: lowercase letters, numbers, hyphens, underscores
// Can be empty
var gcpLabelValuePattern = regexp.MustCompile(`^[a-z0-9_-]*$`)

// ValidateRequired returns an error if the value is empty
func ValidateRequired(value any) error {
	if value == nil {
		return fmt.Errorf("this field is required")
	}

	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("this field is required")
		}
	case []string:
		if len(v) == 0 {
			return fmt.Errorf("at least one selection is required")
		}
	}

	return nil
}

// ValidateGCPResourceName validates a GCP resource name
func ValidateGCPResourceName(value any) error {
	name, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value")
	}

	name = strings.TrimSpace(name)
	if name == "" {
		return nil // Let ValidateRequired handle empty values
	}

	if len(name) > 63 {
		return fmt.Errorf("name must be 63 characters or less (got %d)", len(name))
	}

	if !gcpResourceNamePattern.MatchString(name) {
		return fmt.Errorf("name must start with lowercase letter, contain only lowercase letters, numbers, and hyphens")
	}

	return nil
}

// ValidateGCPLabelKey validates a GCP label key
func ValidateGCPLabelKey(value any) error {
	key, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value")
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}

	if len(key) > 63 {
		return fmt.Errorf("label key must be 63 characters or less (got %d)", len(key))
	}

	if !gcpLabelKeyPattern.MatchString(key) {
		return fmt.Errorf("label key must start with lowercase letter, contain only lowercase letters, numbers, hyphens, underscores")
	}

	return nil
}

// ValidateGCPLabelValue validates a GCP label value
func ValidateGCPLabelValue(value any) error {
	val, ok := value.(string)
	if !ok {
		return fmt.Errorf("expected string value")
	}

	val = strings.TrimSpace(val)
	if val == "" {
		return nil // Empty values are allowed
	}

	if len(val) > 63 {
		return fmt.Errorf("label value must be 63 characters or less (got %d)", len(val))
	}

	if !gcpLabelValuePattern.MatchString(val) {
		return fmt.Errorf("label value must contain only lowercase letters, numbers, hyphens, underscores")
	}

	return nil
}

// ValidateNumber returns a validator that checks if a number is within a range
func ValidateNumber(min, max int64) Validator {
	return func(value any) error {
		var num int64
		switch v := value.(type) {
		case int:
			num = int64(v)
		case int64:
			num = v
		case float64:
			num = int64(v)
		case string:
			return fmt.Errorf("expected numeric value")
		default:
			return fmt.Errorf("expected numeric value")
		}

		if num < min {
			return fmt.Errorf("value must be at least %d", min)
		}
		if num > max {
			return fmt.Errorf("value must be at most %d", max)
		}

		return nil
	}
}

// ValidateStringLength returns a validator that checks string length
func ValidateStringLength(min, max int) Validator {
	return func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		length := len(str)
		if length < min {
			return fmt.Errorf("must be at least %d characters (got %d)", min, length)
		}
		if length > max {
			return fmt.Errorf("must be at most %d characters (got %d)", max, length)
		}

		return nil
	}
}

// ValidatePattern returns a validator that checks against a regex pattern
func ValidatePattern(pattern string, errorMsg string) Validator {
	re := regexp.MustCompile(pattern)
	return func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		if str == "" {
			return nil // Let ValidateRequired handle empty values
		}

		if !re.MatchString(str) {
			if errorMsg != "" {
				return fmt.Errorf("%s", errorMsg)
			}
			return fmt.Errorf("value does not match required pattern")
		}

		return nil
	}
}

// ValidateEmail returns a validator for email addresses
func ValidateEmail() Validator {
	// Simple email pattern - not RFC 5322 compliant but catches most issues
	emailPattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	return ValidatePattern(emailPattern, "invalid email address")
}

// ValidateURL returns a validator for URLs
func ValidateURL() Validator {
	urlPattern := `^https?://[^\s]+$`
	return ValidatePattern(urlPattern, "invalid URL (must start with http:// or https://)")
}

// ValidateIPAddress returns a validator for IPv4 addresses
func ValidateIPAddress() Validator {
	ipPattern := `^(\d{1,3}\.){3}\d{1,3}$`
	return func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		if str == "" {
			return nil
		}

		re := regexp.MustCompile(ipPattern)
		if !re.MatchString(str) {
			return fmt.Errorf("invalid IP address format")
		}

		// Validate each octet is 0-255
		parts := strings.Split(str, ".")
		for _, part := range parts {
			var num int
			_, _ = fmt.Sscanf(part, "%d", &num)
			if num < 0 || num > 255 {
				return fmt.Errorf("invalid IP address: octet %s out of range", part)
			}
		}

		return nil
	}
}

// ValidateCIDR returns a validator for CIDR notation (e.g., 10.0.0.0/8)
func ValidateCIDR() Validator {
	cidrPattern := `^(\d{1,3}\.){3}\d{1,3}/\d{1,2}$`
	return func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		if str == "" {
			return nil
		}

		re := regexp.MustCompile(cidrPattern)
		if !re.MatchString(str) {
			return fmt.Errorf("invalid CIDR notation (expected format: x.x.x.x/xx)")
		}

		// Validate the IP part
		parts := strings.Split(str, "/")
		ipValidator := ValidateIPAddress()
		if err := ipValidator(parts[0]); err != nil {
			return err
		}

		// Validate the prefix length
		var prefix int
		_, _ = fmt.Sscanf(parts[1], "%d", &prefix)
		if prefix < 0 || prefix > 32 {
			return fmt.Errorf("invalid CIDR prefix length: must be 0-32")
		}

		return nil
	}
}

// ValidateOneOf returns a validator that checks if value is one of allowed values
func ValidateOneOf(allowed []string) Validator {
	return func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		if str == "" {
			return nil
		}

		for _, a := range allowed {
			if str == a {
				return nil
			}
		}

		return fmt.Errorf("value must be one of: %s", strings.Join(allowed, ", "))
	}
}

// ValidateNotOneOf returns a validator that checks if value is NOT one of disallowed values
func ValidateNotOneOf(disallowed []string, errorMsg string) Validator {
	return func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		for _, d := range disallowed {
			if str == d {
				if errorMsg != "" {
					return fmt.Errorf("%s", errorMsg)
				}
				return fmt.Errorf("value '%s' is not allowed", str)
			}
		}

		return nil
	}
}

// ComposeValidators chains multiple validators together
// All validators run, and the first error is returned
func ComposeValidators(validators ...Validator) Validator {
	return func(value any) error {
		for _, v := range validators {
			if v == nil {
				continue
			}
			if err := v(value); err != nil {
				return err
			}
		}
		return nil
	}
}

// ValidateAll runs all validators and collects all errors
func ValidateAll(validators ...Validator) func(value any) []error {
	return func(value any) []error {
		var errors []error
		for _, v := range validators {
			if v == nil {
				continue
			}
			if err := v(value); err != nil {
				errors = append(errors, err)
			}
		}
		return errors
	}
}

// ConditionalValidator runs a validator only if a condition is met
func ConditionalValidator(condition func(value any) bool, validator Validator) Validator {
	return func(value any) error {
		if condition(value) {
			return validator(value)
		}
		return nil
	}
}

// ValidateNotEmpty is a simpler alias for ValidateRequired
func ValidateNotEmpty(value any) error {
	return ValidateRequired(value)
}

// ValidateDiskSize validates a disk size value (GCP constraints)
func ValidateDiskSize(minGB, maxGB int64) Validator {
	return func(value any) error {
		// Get the numeric value
		var sizeGB int64
		switch v := value.(type) {
		case int:
			sizeGB = int64(v)
		case int64:
			sizeGB = v
		case float64:
			sizeGB = int64(v)
		default:
			return fmt.Errorf("expected numeric disk size")
		}

		if sizeGB < minGB {
			return fmt.Errorf("disk size must be at least %d GB", minGB)
		}
		if sizeGB > maxGB {
			return fmt.Errorf("disk size must be at most %d GB (GCP limit: 65536 GB)", maxGB)
		}

		return nil
	}
}
