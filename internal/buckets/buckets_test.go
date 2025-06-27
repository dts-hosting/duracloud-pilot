package buckets

import (
	"context"
	"duracloud/internal/accounts"
	"strings"
	"testing"
)

// createTestContext creates a context with AWS context for testing
func createTestContext(stackName string) context.Context {
	awsCtx := accounts.AWSContext{
		AccountID: "123456789012",
		Region:    "us-east-1",
		StackName: stackName,
	}
	return context.WithValue(context.Background(), accounts.AWSContextKey, awsCtx)
}

func TestValidateBucketName(t *testing.T) {
	testStackName := "test-stack"
	ctx := createTestContext(testStackName)
	maxBucketLength := BucketNameMaxChars - (len(testStackName) + len(ReplicationSuffix))

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
			name:     "valid at max length",
			input:    strings.Repeat("a", maxBucketLength),
			expected: true,
			reason:   "bucket name at maximum allowed length accounting for replication suffix",
		},

		// Invalid cases
		{
			name:     "invalid empty string",
			input:    "",
			expected: false,
			reason:   "empty string is below minimum length of 1",
		},
		{
			name:     "invalid ends with hyphen",
			input:    "my-bucket-",
			expected: false,
			reason:   "bucket names cannot end with hyphen",
		},
		{
			name:     "invalid starts with hyphen",
			input:    "-my-bucket",
			expected: false,
			reason:   "bucket names cannot start with hyphen",
		},
		{
			name:     "invalid single hyphen",
			input:    "-",
			expected: false,
			reason:   "single hyphen starts and ends with hyphen",
		},
		{
			name:     "underscore character",
			input:    "my_bucket",
			expected: false,
			reason:   "underscores are not allowed",
		},
		{
			name:     "space character",
			input:    "my bucket",
			expected: false,
			reason:   "spaces are not allowed",
		},
		{
			name:     "uppercase character",
			input:    "My-Bucket",
			expected: false,
			reason:   "uppercase characters are not allowed",
		},
		{
			name:     "invalid over max length",
			input:    strings.Repeat("a", maxBucketLength+1),
			expected: false,
			reason:   "bucket name exceeds maximum length (accounting for replication suffix)",
		},

		// Reserved prefixes
		{
			name:     "aws prefix",
			input:    "aws-my-bucket",
			expected: false,
			reason:   "aws- prefix is reserved",
		},
		{
			name:     "duracloud prefix",
			input:    "duracloud-bucket",
			expected: false,
			reason:   "duracloud- prefix is reserved",
		},

		// Reserved suffixes
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
			input:    "backup-repl",
			expected: false,
			reason:   "repl suffix is reserved",
		},

		// Partial matches (should be valid)
		{
			name:     "managed prefix not suffix",
			input:    "managed-bucket",
			expected: true,
			reason:   "managed as prefix, not suffix, should be allowed",
		},
		{
			name:     "contains aws not as prefix",
			input:    "my-aws-bucket",
			expected: true,
			reason:   "contains aws but doesn't start with it",
		},
		{
			name:     "contains replication not as suffix",
			input:    "replication-bucket",
			expected: true,
			reason:   "contains replication but doesn't end with -repl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateBucketName(ctx, tt.input)
			if result != tt.expected {
				t.Errorf("ValidateBucketName(%q) = %v, want %v (%s)",
					tt.input, result, tt.expected, tt.reason)
			}
		})
	}
}

func TestValidateBucketNameNoContext(t *testing.T) {
	// Test behavior when AWS context is not available
	result := ValidateBucketName(context.Background(), "test-bucket")
	if result != false {
		t.Errorf("ValidateBucketName without AWS context should return false, got %v", result)
	}
}

func TestHasReservedPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"normal bucket name", "my-bucket", false},
		{"aws prefix", "aws-my-bucket", true},
		{"duracloud prefix", "duracloud-service", true},
		{"hyphen prefix", "-my-bucket", true},
		{"contains aws not as prefix", "my-aws-bucket", false},
		{"case sensitive AWS", "AWS-bucket", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasReservedPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("HasReservedPrefix(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHasReservedSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"no reserved suffix", "my-bucket", false},
		{"logs suffix", "access-logs", true},
		{"managed suffix", "system-managed", true},
		{"replication suffix", "backup-repl", true},
		{"bucket-requested suffix", "test-bucket-requested", true},
		{"hyphen suffix", "my-bucket-", true},
		{"managed prefix not suffix", "managed-bucket", false},
		{"case sensitive", "test-LOGS", false},
		{"old replication suffix not detected", "backup-replication", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasReservedSuffix(tt.input)
			if result != tt.expected {
				t.Errorf("HasReservedSuffix(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAllReservedSuffixesCovered(t *testing.T) {
	ctx := createTestContext("test")

	for _, suffix := range ReservedSuffixes {
		t.Run("suffix_"+suffix, func(t *testing.T) {
			// Test the suffix itself
			if !HasReservedSuffix(suffix) {
				t.Errorf("HasReservedSuffix(%q) should return true for the suffix itself", suffix)
			}

			// Test with a prefix (skip for single hyphen to avoid duplication)
			if suffix != "-" {
				testName := "test" + suffix
				if !HasReservedSuffix(testName) {
					t.Errorf("HasReservedSuffix(%q) should return true", testName)
				}

				if ValidateBucketName(ctx, testName) {
					t.Errorf("ValidateBucketName(%q) should return false for reserved suffix", testName)
				}
			}
		})
	}
}

func TestAllReservedPrefixesCovered(t *testing.T) {
	ctx := createTestContext("test")

	for _, prefix := range ReservedPrefixes {
		t.Run("prefix_"+prefix, func(t *testing.T) {
			// Test the prefix itself
			if !HasReservedPrefix(prefix) {
				t.Errorf("HasReservedPrefix(%q) should return true for the prefix itself", prefix)
			}

			// Test with a suffix (skip for single hyphen to avoid duplication)
			if prefix != "-" {
				testName := prefix + "bucket"
				if !HasReservedPrefix(testName) {
					t.Errorf("HasReservedPrefix(%q) should return true", testName)
				}

				if ValidateBucketName(ctx, testName) {
					t.Errorf("ValidateBucketName(%q) should return false for reserved prefix", testName)
				}
			}
		})
	}
}

func TestBucketNameMaxLengthWithReplicationSuffix(t *testing.T) {
	// Test that bucket name length accounts for both stack name AND replication suffix
	tests := []struct {
		stackName string
		maxName   int
	}{
		{"short", 63 - (5 + 5)},                 // 63 - 5 (stack) - 5 (-repl) = 53
		{"medium-stack", 63 - (12 + 5)},         // 63 - 12 (stack) - 5 (-repl) = 46
		{"very-long-stack-name", 63 - (20 + 5)}, // 63 - 20 (stack) - 5 (-repl) = 40
	}

	for _, tt := range tests {
		t.Run("stack_"+tt.stackName, func(t *testing.T) {
			ctx := createTestContext(tt.stackName)

			// Test valid name at max length
			validName := strings.Repeat("a", tt.maxName)
			if !ValidateBucketName(ctx, validName) {
				t.Errorf("ValidateBucketName should accept name of length %d with stack %q (accounting for replication suffix)", tt.maxName, tt.stackName)
			}

			// Test invalid name over max length
			invalidName := strings.Repeat("a", tt.maxName+1)
			if ValidateBucketName(ctx, invalidName) {
				t.Errorf("ValidateBucketName should reject name of length %d with stack %q (accounting for replication suffix)", tt.maxName+1, tt.stackName)
			}
		})
	}
}
