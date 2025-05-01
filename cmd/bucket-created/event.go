package main

import "strings"

type BucketCreatedEvent struct {
	Detail Detail `json:"detail"`
}

type Detail struct {
	RequestParameters RequestParameters `json:"requestParameters"`
}

type RequestParameters struct {
	BucketName string `json:"bucketName"`
}

func (e *BucketCreatedEvent) BucketName() string {
	return e.Detail.RequestParameters.BucketName
}

func (e *BucketCreatedEvent) IsEventLogsBucket() bool {
	return strings.Contains(e.BucketName(), "-event-logs")
}

func (e *BucketCreatedEvent) IsManagedBucket() bool {
	return strings.Contains(e.BucketName(), "-managed")
}

func (e *BucketCreatedEvent) IsPublicBucket() bool {
	return strings.Contains(e.BucketName(), "-public")
}

func (e *BucketCreatedEvent) IsReplicationBucket() bool {
	return strings.Contains(e.BucketName(), "-replication")
}

func (e *BucketCreatedEvent) IsRestrictedBucket() bool {
	return e.IsEventLogsBucket() || e.IsManagedBucket() || e.IsReplicationBucket()
}
