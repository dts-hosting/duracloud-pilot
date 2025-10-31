package db

import (
	"duracloud/internal/files"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

func ExtractBucketAndObject(record events.DynamoDBEventRecord) (files.S3Object, error) {
	bucket, exists := record.Change.OldImage[string(ChecksumTableBucketNameId)]
	if !exists {
		return files.S3Object{}, fmt.Errorf("missing bucket name attribute")
	}

	object, exists := record.Change.OldImage[string(ChecksumTableObjectKeyId)]
	if !exists {
		return files.S3Object{}, fmt.Errorf("missing object key attribute")
	}

	return files.S3Object{Bucket: bucket.String(), Key: object.String()}, nil
}

func IsTTLExpiry(record events.DynamoDBEventRecord) bool {
	return record.EventName == "REMOVE" &&
		record.UserIdentity != nil &&
		record.UserIdentity.Type == "Service" &&
		record.UserIdentity.PrincipalID == "dynamodb.amazonaws.com"
}
