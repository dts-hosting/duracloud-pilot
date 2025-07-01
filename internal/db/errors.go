package db

import (
	"errors"
	"fmt"
)

var (
	ErrChecksumRecordNotFound = errors.New("checksum record not found")
	ErrJitterGeneration       = errors.New("jitter generation failed")
	ErrUnmarshallingChecksum  = errors.New("failed to unmarshal checksum record")
)

func ErrorChecksumRecordNotFound(bucket, key string) error {
	return fmt.Errorf("%w: bucket=%s key=%s", ErrChecksumRecordNotFound, bucket, key)
}

func ErrorGeneratingJitter(jitterType string, cause error) error {
	return fmt.Errorf("%w: type=%s cause=%v", ErrJitterGeneration, jitterType, cause)
}

func ErrorUnmarshallingChecksum(cause error) error {
	return fmt.Errorf("%w: cause=%v", ErrUnmarshallingChecksum, cause)
}
