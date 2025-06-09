package helpers

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type AWSContext struct {
	AccountID string
	Region    string
}

type contextKey string

const AWSContextKey contextKey = "awsContext"

func GetAccountID(ctx context.Context, awsConfig aws.Config) (string, error) {
	stsClient := sts.NewFromConfig(awsConfig)

	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}

	return *result.Account, nil
}
