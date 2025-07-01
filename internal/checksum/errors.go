package checksum

import (
	"errors"
	"fmt"
)

var (
	ErrBytesCountDoesNotMatch = errors.New("bytes expected count does not match bytes read")
	ErrMaxFileSizeExceeded    = errors.New("max file size exceeded")
	ErrMetadataNotRetrieved   = errors.New("metadata not retrieved")
	ErrObjectNotFound         = errors.New("object not found")
	ErrObjectNotRetrieved     = errors.New("object not retrieved")
	ErrReadingFromStream      = errors.New("failed to read from stream")
)

func ErrorBytesCountDoesNotMatch(uri string, bytesExpected int64, bytesRead int64) error {
	return fmt.Errorf("%w: %s expected=%d read=%d",
		ErrBytesCountDoesNotMatch,
		uri,
		bytesExpected,
		bytesRead,
	)
}

func ErrorMaxFileSizeExceeded(uri string, fileSize int64) error {
	return fmt.Errorf("%w: %s=%d bytes (%.2f GB) max=%d bytes (%.2f GB)",
		ErrMaxFileSizeExceeded,
		uri,
		fileSize,
		float64(fileSize)/(1024*1024*1024),
		MaxFileSize,
		float64(MaxFileSize)/(1024*1024*1024),
	)
}

func ErrorMetadataNotRetrieved(uri string, cause error) error {
	return fmt.Errorf("%w: uri=%s cause=%v", ErrMetadataNotRetrieved, uri, cause)
}

func ErrorObjectNotFound(uri string) error {
	return fmt.Errorf("%w: uri=%s", ErrObjectNotFound, uri)
}

func ErrorObjectNotRetrieved(uri string, cause error) error {
	return fmt.Errorf("%w: uri=%s cause=%v", ErrObjectNotRetrieved, uri, cause)
}

func ErrorReadingFromStream(uri string, cause error) error {
	return fmt.Errorf("%w: uri=%s cause=%v", ErrReadingFromStream, uri, cause)
}
