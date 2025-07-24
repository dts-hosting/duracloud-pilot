package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackDeployment(t *testing.T) {
	t.Parallel()

	clients, stackName := setupTestClients(t)
	ctx := context.Background()

	t.Run("VerifyLambdaFunction", func(t *testing.T) {
		functionName := fmt.Sprintf("%s-BucketRequestedFunction", stackName)
		exists := lambdaFunctionExists(ctx, clients.Lambda, functionName)
		assert.True(t, exists, "Lambda function %s should exist", functionName)
	})

	t.Run("VerifyS3Buckets", func(t *testing.T) {
		expectedBuckets := []string{
			fmt.Sprintf("%s-bucket-requested", stackName),
			fmt.Sprintf("%s-managed", stackName),
			fmt.Sprintf("%s-logs", stackName),
		}

		for _, bucketName := range expectedBuckets {
			t.Run(fmt.Sprintf("Bucket_%s", bucketName), func(t *testing.T) {
				exists := bucketExists(ctx, clients.S3, bucketName)
				assert.True(t, exists, "Bucket %s should exist", bucketName)
			})
		}
	})

	t.Run("VerifyManagedBucketConfiguration", func(t *testing.T) {
		managedBucket := fmt.Sprintf("%s-managed", stackName)

		// Check lifecycle configuration
		lifecycle := getBucketLifecycle(ctx, clients.S3, managedBucket)
		require.NotNil(t, lifecycle, "Managed bucket should have lifecycle configuration")
		assert.NotEmpty(t, lifecycle.Rules, "Managed bucket should have lifecycle rules")

		// Verify the 30-day deletion rule
		found30DayRule := false
		for _, rule := range lifecycle.Rules {
			if rule.Expiration != nil && rule.Expiration.Days != nil && *rule.Expiration.Days == 30 {
				found30DayRule = true
				break
			}
		}
		assert.True(t, found30DayRule, "Managed bucket should have 30-day expiration rule")
	})

	t.Run("VerifyIAMRoles", func(t *testing.T) {
		expectedRoles := []string{
			fmt.Sprintf("%s-s3-replication-role", stackName),
			fmt.Sprintf("%s-invoke-sqs-role", stackName),
		}

		for _, roleName := range expectedRoles {
			t.Run(fmt.Sprintf("Role_%s", roleName), func(t *testing.T) {
				exists := iamRoleExists(ctx, clients.IAM, roleName)
				assert.True(t, exists, "IAM role %s should exist", roleName)
			})
		}
	})
}
