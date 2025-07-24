# DuraCloud Pilot Development Guidelines

This document provides essential information for developers working on the DuraCloud Pilot project.

## Build and Configuration Instructions

### Prerequisites

- Go 1.24 or later
- AWS CLI configured with appropriate credentials
- AWS SAM CLI
- Docker
- Node.js (for documentation site)

### Environment Setup

1. Create a `.env` file in the project root with required environment variables:

```
AWS_PROFILE=your-profile-name
```

2. Pull required Docker images:

```bash
make pull
```

### Building the Project

The project uses AWS SAM for building and deploying serverless functions:

```bash
# Build the project
make build

# Deploy to AWS
make deploy stack=your-stack-name
```

### Lambda Architecture

The project supports both x86_64 and arm64 architectures for Lambda functions. The architecture can be specified during build and deployment:

```bash
# For ARM64
make build LambdaArchitecture=arm64
make deploy stack=your-stack-name LambdaArchitecture=arm64

# For x86_64 (default)
make build LambdaArchitecture=x86_64
make deploy stack=your-stack-name LambdaArchitecture=x86_64
```

### Documentation

The project includes a documentation site built with Astro:

```bash
# Install documentation dependencies
make docs-install

# Start the documentation development server
make docs-dev

# Build the documentation site
make docs-build
```

## Testing Information

### Unit Tests

Unit tests are written using Go's standard testing package. They typically use mocks to avoid external dependencies.

To run unit tests for a specific package:

```bash
cd path/to/package
go test -v
```

Example of a unit test:

```
func TestFormatBytes(t *testing.T) {
    tests := []struct {
        name     string
        bytes    int64
        expected string
    }{
        {
            name:     "kilobytes",
            bytes:    1500,
            expected: "1.46 KB",
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := FormatBytes(tt.bytes)
            if result != tt.expected {
                t.Errorf("FormatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
            }
        })
    }
}
```

### Integration Tests

Integration tests require AWS resources and credentials. They are located in the `tests/integration` directory.

To run integration tests:

```bash
# Set the stack name environment variable
export STACK_NAME=your-stack-name

# Run all tests and clean up resources
make test stack=your-stack-name
```

Integration tests will:

1. Create necessary AWS resources
2. Run the tests against those resources
3. Clean up the resources after tests complete

### Adding New Tests

#### Unit Tests

1. Create a file named `*_test.go` in the same package as the code being tested
2. Use table-driven tests for comprehensive test coverage
3. Mock external dependencies (like AWS services) using interfaces
4. Follow the existing patterns for error handling and assertions

#### Integration Tests

1. Add new test functions to existing files in `tests/integration` or create new files as needed
2. Use the helper functions in `tests/integration/helpers.go` for common operations
3. Ensure proper cleanup of resources in test cleanup functions
4. Set appropriate timeouts for operations that may take time to complete

## Code Style and Development Guidelines

### Go Code Style

- Follow standard Go code style and conventions
- Use meaningful variable and function names
- Document exported functions, types, and constants
- Use interfaces for dependency injection to improve testability
- Handle errors explicitly and return them to the caller when appropriate
- Use context for cancellation and timeouts

### Project Structure

- `cmd/`: Contains Lambda function entry points
- `internal/`: Contains packages used internally by the project
- `events/`: Contains sample events for testing Lambda functions
- `tests/`: Contains integration tests
- `docs/`: Contains the documentation site
- `scripts/`: Contains utility scripts for development and deployment

### AWS Resources

- Lambda functions are defined in `template.yaml`
- S3 buckets follow a naming convention: `{stack-name}-{purpose}`
- Replication buckets have a suffix defined in the `buckets` package

### Error Handling

- Create custom error types for specific error conditions
- Use descriptive error messages that include relevant context
- Log errors with appropriate severity levels

### Logging

- Use structured logging (JSON format)
- Include relevant context in log messages
- Use appropriate log levels (info, warning, error)

## Debugging

### Local Debugging

Use the SAM CLI to invoke functions locally:

```bash
make invoke func=FunctionName event=path/to/event.json
```

### Remote Debugging

View logs from deployed functions:

```bash
make logs func=FunctionName stack=your-stack-name interval=5m
```

Invoke a deployed function remotely:

```bash
make invoke-remote func=FunctionName event=path/to/event.json stack=your-stack-name
```

### Common Issues

1. **Missing AWS Credentials**: Ensure AWS credentials are properly configured
2. **Permissions Issues**: Check IAM roles and policies in `template.yaml`
3. **Resource Limits**: Be aware of AWS service limits, especially for S3 operations
4. **Deployment Failures**: Check CloudFormation events for detailed error messages

## Regenerating These Guidelines

To regenerate these development guidelines, you can use the following prompt:

```
Please record in `guidelines.md` any relevant details that will aid future development on this project. This should include, but is not limited to:

1. **Build/Configuration Instructions**: If specific build or configuration steps are required, provide clear and detailed instructions for setting up the project.

2. **Testing Information**:
   - Instructions for configuring and running tests.
   - Guidelines on adding and executing new tests.
   - Create and run a simple test to demonstrate the process.

3. **Additional Development Information**: Information about the code style or any other information you believe would be useful for the development or debugging process.

Important Notes:
- This information is intended for an advanced developer; there's no need to include basic things, only information specific to this project.
- Check that the test examples work before writing the information to the file.
```

The guidelines should be updated whenever significant changes are made to the build process, testing framework, or development practices.
