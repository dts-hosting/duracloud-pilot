package buckets

import (
	"context"
	"duracloud/internal/accounts"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type BucketRequest struct {
	ctx                context.Context
	name               string
	prefix             string
	managedBucketName  string
	replicationRoleArn string
	resultChan         chan<- map[string]string
	s3Client           *s3.Client
}

func NewBucketRequest(
	ctx context.Context, s3Client *s3.Client,
	name, prefix, managedBucketName, replicationRoleArn string,
	resultChan chan<- map[string]string,
) *BucketRequest {
	return &BucketRequest{
		ctx:                ctx,
		name:               name,
		prefix:             prefix,
		managedBucketName:  managedBucketName,
		replicationRoleArn: replicationRoleArn,
		resultChan:         resultChan,
		s3Client:           s3Client,
	}
}

func (b *BucketRequest) AddBucketTags(name, bucketType string) error {
	_, err := b.s3Client.PutBucketTagging(b.ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(name),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String(ApplicationTagKey), Value: aws.String(ApplicationTagValue)},
				{Key: aws.String(StackNameTagKey), Value: aws.String(b.prefix)},
				{Key: aws.String(BucketTypeTagKey), Value: aws.String(bucketType)},
			},
		},
	})
	if err != nil {
		return ErrorApplyingBucketTags(err)
	}
	return nil
}

func (b *BucketRequest) AddDenyUploadPolicy(name string) error {
	// apply a default deny-upload policy
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":       "DenyAllUploads",
				"Effect":    "Deny",
				"Principal": "*",
				"Action":    "s3:PutObject",
				"Resource":  fmt.Sprintf("arn:aws:s3:::%s/*", name),
			},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return ErrorMarshallingPolicy(err)
	}

	_, err = b.s3Client.PutBucketPolicy(b.ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(name),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return ErrorApplyingBucketPolicy(err)
	}
	return nil
}

func (b *BucketRequest) AddExpiration(name string) error {
	daysToExpiration := int32(NonCurrentVersionExpirationDays)
	daysToAbortMultipart := int32(AbortIncompleteMultipartDays)

	_, err := b.s3Client.PutBucketLifecycleConfiguration(b.ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(name),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("ExpireOldVersions"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")},
					NoncurrentVersionExpiration: &types.NoncurrentVersionExpiration{
						NoncurrentDays: &daysToExpiration,
					},
					AbortIncompleteMultipartUpload: &types.AbortIncompleteMultipartUpload{
						DaysAfterInitiation: &daysToAbortMultipart,
					},
				},
			},
		},
	})
	if err != nil {
		return ErrorApplyingExpiration(err)
	}
	return nil
}

func (b *BucketRequest) AddLifecycle(name string, storageClass types.TransitionStorageClass, transitionDays int32) error {
	_, err := b.s3Client.PutBucketLifecycleConfiguration(b.ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(name),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String(fmt.Sprintf("TransitionTo%s", storageClass)),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("")},
					Transitions: []types.Transition{
						{
							Days:         &transitionDays,
							StorageClass: storageClass,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return ErrorApplyingLifecycle(err)
	}
	return nil
}

func (b *BucketRequest) AddPublicPolicy(name string) error {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Sid":       "AllowPublicRead",
				"Effect":    "Allow",
				"Principal": "*",
				"Action":    "s3:GetObject",
				"Resource":  fmt.Sprintf("arn:aws:s3:::%s/*", name),
			},
		},
	}

	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return ErrorMarshallingBucketPolicy(err)
	}

	_, err = b.s3Client.PutBucketPolicy(b.ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(name),
		Policy: aws.String(string(policyJSON)),
	})
	if err != nil {
		return ErrorApplyingBucketPolicy(err)
	}
	return nil
}

func (b *BucketRequest) AddReplicationLifecycle(name string) error {
	daysToGlacier := int32(LifeCycleTransitionToGlacierDays)
	return b.AddLifecycle(name, types.TransitionStorageClassDeepArchive, daysToGlacier)
}

func (b *BucketRequest) AddStandardLifecycle(name string) error {
	daysToGlacier := int32(LifeCycleTransitionToGlacierDays)
	return b.AddLifecycle(name, types.TransitionStorageClassGlacierIr, daysToGlacier)
}

func (b *BucketRequest) CreateNewBucket(name string) error {
	awsCtx, ok := b.ctx.Value(accounts.AWSContextKey).(accounts.AWSContext)
	if !ok {
		return ErrorAWSContextRetrieval()
	}

	_, err := b.s3Client.CreateBucket(b.ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(awsCtx.Region),
		},
	})
	if err != nil {
		return ErrorBucketCreationFailed(err)
	}
	return nil
}

func (b *BucketRequest) DeleteBucket(name string) error {
	_, err := b.s3Client.DeleteBucket(b.ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return ErrorBucketDeletionFailed(err)
	}
	return nil
}

func (b *BucketRequest) EnableEventBridge(name string) error {
	_, err := b.s3Client.PutBucketNotificationConfiguration(b.ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket: aws.String(name),
		NotificationConfiguration: &types.NotificationConfiguration{
			EventBridgeConfiguration: &types.EventBridgeConfiguration{},
		},
	})
	if err != nil {
		return ErrorApplyingEventBridge(err)
	}
	return nil
}

func (b *BucketRequest) EnableInventory(srcName string, destName string) error {
	awsCtx, ok := b.ctx.Value(accounts.AWSContextKey).(accounts.AWSContext)
	if !ok {
		return ErrorAWSContextRetrieval()
	}

	_, err := b.s3Client.PutBucketInventoryConfiguration(b.ctx, &s3.PutBucketInventoryConfigurationInput{
		Bucket: aws.String(srcName),
		Id:     aws.String(InventoryConfigId),
		InventoryConfiguration: &types.InventoryConfiguration{
			IsEnabled:              aws.Bool(true),
			Id:                     aws.String(InventoryConfigId),
			IncludedObjectVersions: types.InventoryIncludedObjectVersionsAll,
			Schedule: &types.InventorySchedule{
				Frequency: types.InventoryFrequencyDaily,
			},
			Destination: &types.InventoryDestination{
				S3BucketDestination: &types.InventoryS3BucketDestination{
					AccountId: aws.String(awsCtx.AccountID),
					Bucket:    aws.String(fmt.Sprintf("arn:aws:s3:::%s", destName)),
					Format:    types.InventoryFormatCsv,
					Prefix:    aws.String(InventoryConfigId),
				},
			},
			OptionalFields: []types.InventoryOptionalField{
				types.InventoryOptionalFieldSize,
				types.InventoryOptionalFieldLastModifiedDate,
				types.InventoryOptionalFieldStorageClass,
				types.InventoryOptionalFieldReplicationStatus,
			},
		},
	})

	if err != nil {
		return ErrorApplyingInventory(err)
	}
	return nil
}

func (b *BucketRequest) EnableLogging(srcName string, destName string) error {
	_, err := b.s3Client.PutBucketLogging(b.ctx, &s3.PutBucketLoggingInput{
		Bucket: aws.String(srcName),
		BucketLoggingStatus: &types.BucketLoggingStatus{
			LoggingEnabled: &types.LoggingEnabled{
				TargetBucket: aws.String(destName),
				TargetPrefix: aws.String(fmt.Sprintf("audit/%s/", srcName)),
			},
		},
	})
	if err != nil {
		return ErrorApplyingLogging(err)
	}
	return nil
}

func (b *BucketRequest) EnableReplication(srcName string, replName string, replRoleArn string) error {
	_, err := b.s3Client.PutBucketReplication(b.ctx, &s3.PutBucketReplicationInput{
		Bucket: aws.String(srcName),
		ReplicationConfiguration: &types.ReplicationConfiguration{
			Role: aws.String(replRoleArn),
			Rules: []types.ReplicationRule{
				{
					ID:       aws.String("ReplicateAll"),
					Status:   types.ReplicationRuleStatusEnabled,
					Priority: aws.Int32(1),
					Filter:   &types.ReplicationRuleFilter{Prefix: aws.String("")},
					Destination: &types.Destination{
						Bucket: aws.String(fmt.Sprintf("arn:aws:s3:::%s", replName)),
						ReplicationTime: &types.ReplicationTime{
							Status: types.ReplicationTimeStatusEnabled,
							Time: &types.ReplicationTimeValue{
								Minutes: aws.Int32(15),
							},
						},
						Metrics: &types.Metrics{
							Status: types.MetricsStatusEnabled,
							EventThreshold: &types.ReplicationTimeValue{
								Minutes: aws.Int32(15),
							},
						},
					},
					DeleteMarkerReplication: &types.DeleteMarkerReplication{
						Status: types.DeleteMarkerReplicationStatusEnabled,
					},
				},
			},
		},
	})

	if err != nil {
		return ErrorApplyingReplication(err)
	}
	return nil
}

func (b *BucketRequest) EnableVersioning(name string) error {
	_, err := b.s3Client.PutBucketVersioning(b.ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(name),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return ErrorApplyingVersioning(err)
	}
	return nil
}

func (b *BucketRequest) FullName() string {
	return fmt.Sprintf("%s-%s", b.prefix, b.name)
}

func (b *BucketRequest) MakePublic(name string) error {
	blockFalse := false
	_, err := b.s3Client.PutPublicAccessBlock(b.ctx, &s3.PutPublicAccessBlockInput{
		Bucket: aws.String(name),
		PublicAccessBlockConfiguration: &types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       &blockFalse,
			IgnorePublicAcls:      &blockFalse,
			BlockPublicPolicy:     &blockFalse,
			RestrictPublicBuckets: &blockFalse,
		},
	})
	if err != nil {
		return ErrorApplyingPublicAccessBlock(err)
	}
	return nil
}

func (b *BucketRequest) RemovePolicy(name string) error {
	_, err := b.s3Client.DeleteBucketPolicy(b.ctx, &s3.DeleteBucketPolicyInput{
		Bucket: aws.String(name),
	})

	if err != nil {
		return ErrorDeletingBucketPolicy(err)
	}
	return nil
}

func (b *BucketRequest) ReplicationName() string {
	return fmt.Sprintf("%s%s", b.FullName(), ReplicationSuffix)
}

func (b *BucketRequest) Setup() {
	localStatus := make(map[string]string)
	fullBucketName := b.FullName()
	replicationBucketName := b.ReplicationName()

	var (
		mainBucketCreated bool
		replBucketCreated bool
		setupComplete     bool
	)

	// Cleanup function runs on any exit
	defer func() {
		if !setupComplete {
			if mainBucketCreated {
				if err := b.DeleteBucket(fullBucketName); err != nil {
					log.Printf("WARNING: Failed to cleanup bucket %s: %v", fullBucketName, err)
				}
			}
			if replBucketCreated {
				if err := b.DeleteBucket(replicationBucketName); err != nil {
					log.Printf("WARNING: Failed to cleanup bucket %s: %v", replicationBucketName, err)
				}
			}
		}
		b.resultChan <- localStatus
	}()

	log.Printf("Creating buckets: %s [%s]", fullBucketName, replicationBucketName)

	// Create and configure main bucket
	if err := b.CreateNewBucket(fullBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}
	mainBucketCreated = true

	if err := b.AddDenyUploadPolicy(fullBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	if err := b.AddBucketTags(fullBucketName, StandardTagValue); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	if err := b.EnableVersioning(fullBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	if err := b.AddExpiration(fullBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	if IsPublicBucket(fullBucketName) {
		if err := b.MakePublic(fullBucketName); err != nil {
			localStatus[fullBucketName] = err.Error()
			return
		}

		if err := b.AddBucketTags(fullBucketName, PublicTagValue); err != nil {
			localStatus[fullBucketName] = err.Error()
			return
		}
	} else {
		if err := b.AddStandardLifecycle(fullBucketName); err != nil {
			localStatus[fullBucketName] = err.Error()
			return
		}
	}

	if err := b.EnableEventBridge(fullBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	if err := b.EnableInventory(fullBucketName, b.managedBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	if err := b.EnableLogging(fullBucketName, b.managedBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	// Create and configure replication bucket
	if err := b.CreateNewBucket(replicationBucketName); err != nil {
		localStatus[replicationBucketName] = err.Error()
		return
	}
	replBucketCreated = true

	if err := b.AddBucketTags(replicationBucketName, ReplicationTagValue); err != nil {
		localStatus[replicationBucketName] = err.Error()
		return
	}

	if err := b.EnableVersioning(replicationBucketName); err != nil {
		localStatus[replicationBucketName] = err.Error()
		return
	}

	if err := b.AddExpiration(replicationBucketName); err != nil {
		localStatus[replicationBucketName] = err.Error()
		return
	}

	if err := b.EnableReplication(fullBucketName, replicationBucketName, b.replicationRoleArn); err != nil {
		localStatus[replicationBucketName] = err.Error()
		return
	}

	if err := b.AddReplicationLifecycle(replicationBucketName); err != nil {
		localStatus[replicationBucketName] = err.Error()
		return
	}

	if err := b.RemovePolicy(fullBucketName); err != nil {
		localStatus[fullBucketName] = err.Error()
		return
	}

	// Note: we have to do this after removing the temporary DENY policy
	if IsPublicBucket(fullBucketName) {
		if err := b.AddPublicPolicy(fullBucketName); err != nil {
			localStatus[fullBucketName] = err.Error()
			return
		}
	}

	// If we get here, everything succeeded
	setupComplete = true
	localStatus[fullBucketName] = StatusBucketCreatedSuccessfully
}
