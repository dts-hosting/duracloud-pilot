package buckets

import (
	"testing"
)

func TestValidateBucketName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		reason   string
	}{
		// Valid bucket names
		{
			name:     "valid simple name",
			input:    "my-bucket",
			expected: true,
			reason:   "simple valid bucket name",
		},
		{
			name:     "valid with numbers",
			input:    "bucket123",
			expected: true,
			reason:   "bucket name with numbers",
		},
		{
			name:     "valid with hyphens",
			input:    "my-test-bucket-name",
			expected: true,
			reason:   "bucket name with multiple hyphens",
		},
		{
			name:     "valid alphanumeric",
			input:    "abc123def456",
			expected: true,
			reason:   "alphanumeric bucket name",
		},
		{
			name:     "valid single character",
			input:    "a",
			expected: true,
			reason:   "single character bucket name",
		},
		{
			name:     "public suffix",
			input:    "content-public",
			expected: true,
			reason:   "public suffix is ok",
		},

		// Invalid characters
		{
			name:     "underscore character",
			input:    "my_bucket",
			expected: false,
			reason:   "underscores are not allowed",
		},
		{
			name:     "dot character",
			input:    "my.bucket",
			expected: false,
			reason:   "dots are not allowed",
		},
		{
			name:     "space character",
			input:    "my bucket",
			expected: false,
			reason:   "spaces are not allowed",
		},
		{
			name:     "special characters",
			input:    "my@bucket#",
			expected: false,
			reason:   "special characters are not allowed",
		},
		{
			name:     "unicode characters",
			input:    "my-bücket",
			expected: false,
			reason:   "unicode characters are not allowed",
		},

		// Reserved suffixes - should be rejected
		{
			name:     "bucket-requested suffix",
			input:    "test-bucket-requested",
			expected: false,
			reason:   "bucket-requested suffix is reserved",
		},
		{
			name:     "logs suffix",
			input:    "access-logs",
			expected: false,
			reason:   "logs suffix is reserved",
		},
		{
			name:     "managed suffix",
			input:    "system-managed",
			expected: false,
			reason:   "managed suffix is reserved",
		},
		{
			name:     "replication suffix",
			input:    "backup-replication",
			expected: false,
			reason:   "replication suffix is reserved",
		},

		// Partial matches (should be valid)
		{
			name:     "managed prefix not suffix",
			input:    "managed-bucket",
			expected: true,
			reason:   "managed as prefix, not suffix, should be allowed",
		},
		{
			name:     "logs prefix not suffix",
			input:    "logs-archive",
			expected: true,
			reason:   "logs as prefix, not suffix, should be allowed",
		},
		{
			name:     "replication prefix not suffix",
			input:    "replication-source",
			expected: true,
			reason:   "replication as prefix, not suffix, should be allowed",
		},
		{
			name:     "contains reserved word",
			input:    "my-logs-bucket",
			expected: true,
			reason:   "contains logs but doesn't end with it",
		},

		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: true,
			reason:   "empty string has no invalid chars or suffixes",
		},
		{
			name:     "only hyphen",
			input:    "-",
			expected: true,
			reason:   "single hyphen is valid character",
		},
		{
			name:     "exact reserved suffix",
			input:    "-logs",
			expected: false,
			reason:   "exact reserved suffix should be rejected",
		},
		{
			name:     "case sensitivity",
			input:    "test-LOGS",
			expected: true,
			reason:   "reserved suffixes are case sensitive",
		},
		{
			name:     "multiple reserved patterns",
			input:    "logs-managed",
			expected: false,
			reason:   "ends with reserved suffix managed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBucketName(tt.input)
			if result != tt.expected {
				t.Errorf("ValidateBucketName(%q) = %v, want %v (%s)",
					tt.input, result, tt.expected, tt.reason)
			}
		})
	}
}

func TestHasReservedSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
		reason   string
	}{
		// No reserved suffixes
		{
			name:     "no suffix",
			input:    "my-bucket",
			expected: false,
			reason:   "no reserved suffix present",
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
			reason:   "empty string has no suffix",
		},
		{
			name:     "suffix only no hyphen",
			input:    "logs",
			expected: false,
			reason:   "suffix without prefix",
		},
		{
			name:     "public suffix",
			input:    "content-public",
			expected: false,
			reason:   "ends with public suffix",
		},

		// Reserved suffixes present
		{
			name:     "bucket-requested suffix",
			input:    "test-bucket-requested",
			expected: true,
			reason:   "ends with bucket-requested suffix",
		},
		{
			name:     "logs suffix",
			input:    "access-logs",
			expected: true,
			reason:   "ends with logs suffix",
		},
		{
			name:     "managed suffix",
			input:    "system-managed",
			expected: true,
			reason:   "ends with managed suffix",
		},
		{
			name:     "replication suffix",
			input:    "backup-replication",
			expected: true,
			reason:   "ends with replication suffix",
		},
		{
			name:     "exact suffix",
			input:    "-logs",
			expected: true,
			reason:   "exact reserved suffix",
		},

		// Partial matches (not suffixes)
		{
			name:     "managed prefix",
			input:    "managed-bucket",
			expected: false,
			reason:   "managed as prefix, not suffix",
		},
		{
			name:     "logs prefix",
			input:    "logs-archive",
			expected: false,
			reason:   "logs as prefix, not suffix",
		},
		{
			name:     "public prefix",
			input:    "public-data",
			expected: false,
			reason:   "public as prefix, not suffix",
		},
		{
			name:     "replication prefix",
			input:    "replication-source",
			expected: false,
			reason:   "replication as prefix, not suffix",
		},
		{
			name:     "contains but not suffix",
			input:    "my-logs-bucket",
			expected: false,
			reason:   "contains logs but doesn't end with it",
		},
		{
			name:     "similar but different",
			input:    "my-log",
			expected: false,
			reason:   "similar to logs but not exact suffix",
		},
		{
			name:     "partial match",
			input:    "manage",
			expected: false,
			reason:   "partial match of managed suffix",
		},

		// Case sensitivity
		{
			name:     "uppercase suffix",
			input:    "test-LOGS",
			expected: false,
			reason:   "suffixes are case sensitive",
		},
		{
			name:     "mixed case suffix",
			input:    "test-Logs",
			expected: false,
			reason:   "suffixes are case sensitive",
		},
		{
			name:     "uppercase managed",
			input:    "test-MANAGED",
			expected: false,
			reason:   "suffixes are case sensitive",
		},

		// Multiple suffixes or complex cases
		{
			name:     "double suffix",
			input:    "test-logs-managed",
			expected: true,
			reason:   "ends with managed suffix regardless of other content",
		},
		{
			name:     "nested suffix pattern",
			input:    "bucket-requested-logs",
			expected: true,
			reason:   "ends with logs suffix",
		},
		{
			name:     "very long name with suffix",
			input:    "this-is-a-very-long-bucket-name-with-many-hyphens-and-words-logs",
			expected: true,
			reason:   "long name ending with logs suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasReservedSuffix(tt.input)
			if result != tt.expected {
				t.Errorf("HasReservedSuffix(%q) = %v, want %v (%s)",
					tt.input, result, tt.expected, tt.reason)
			}
		})
	}
}

func TestAllReservedSuffixesCovered(t *testing.T) {
	// These should all return true when tested directly as suffixes
	for _, suffix := range ReservedSuffixes {
		t.Run("suffix_"+suffix, func(t *testing.T) {
			// Test the suffix itself
			if !HasReservedSuffix(suffix) {
				t.Errorf("HasReservedSuffix(%q) should return true for the suffix itself", suffix)
			}

			// Test with a prefix
			testName := "test" + suffix
			if !HasReservedSuffix(testName) {
				t.Errorf("HasReservedSuffix(%q) should return true", testName)
			}

			// Test that ValidateBucketName rejects it
			if ValidateBucketName(testName) {
				t.Errorf("ValidateBucketName(%q) should return false for reserved suffix", testName)
			}
		})
	}
}
