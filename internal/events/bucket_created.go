package events

type BucketCreatedEvent struct {
	Detail BucketCreatedDetail `json:"detail"`
}

type BucketCreatedDetail struct {
	RequestParameters BucketCreatedRequestParameters `json:"requestParameters"`
}

type BucketCreatedRequestParameters struct {
	BucketName string `json:"bucketName"`
}

func (e *BucketCreatedEvent) BucketName() string {
	return e.Detail.RequestParameters.BucketName
}
