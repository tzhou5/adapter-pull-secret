# Pull Secret Job - Test Documentation

## Test Coverage Summary

Current test coverage: **42.3%** of statements

## Test Files

- `pull_secret_test.go` - Unit tests for Pull Secret Job

## Tests Included

### 1. Pull Secret Validation (`TestValidatePullSecret`)
Tests the validation of pull secret JSON format:
- ✅ Valid pull secret with single registry
- ✅ Valid pull secret with multiple registries
- ✅ Invalid JSON format
- ✅ Missing 'auths' key
- ✅ Empty auths object
- ✅ Auths is not an object
- ✅ Empty pull secret string

### 2. Configuration Validation (`TestPullSecretTask_validateConfig`)
Tests the validation of required environment variables:
- ✅ Valid configuration with all fields
- ✅ Missing GCP_PROJECT_ID
- ✅ Missing CLUSTER_ID
- ✅ Missing SECRET_NAME
- ✅ Missing PULL_SECRET_DATA
- ✅ All fields empty

### 3. Task Metadata (`TestPullSecretTask_TaskName`)
Tests the task name:
- ✅ Returns "pull-secret-mvp"

### 4. Job Metadata (`TestPullSecretJob_GetMetadata`)
Tests job metadata:
- ✅ Returns correct Use value
- ✅ Returns non-empty Description

### 5. Worker Count (`TestPullSecretJob_GetWorkerCount`)
Tests worker count configuration:
- ✅ Returns 1 worker

### 6. Task Creation (`TestPullSecretJob_GetTasks`)
Tests task creation from environment variables:
- ✅ All environment variables set
- ✅ Auto-generate secret name from cluster ID
- ✅ Use fake pull secret when not provided

### 7. Retry Logic (`TestIsRetryable`)
Tests retry decision for different gRPC error codes:
- ✅ Retryable errors: Unavailable, DeadlineExceeded, Internal, ResourceExhausted
- ✅ Non-retryable errors: PermissionDenied, NotFound, AlreadyExists, InvalidArgument

### 8. Retry with Backoff (`TestRetryWithBackoff`)
Tests the exponential backoff retry mechanism:
- ✅ Success on first try
- ✅ Success after multiple retries
- ✅ Max retries exceeded
- ✅ Non-retryable error stops immediately
- ✅ Context cancellation

### 9. Structured Logging (`TestLogStructured`)
Tests structured JSON logging:
- ✅ Complete log entry with all fields
- ✅ Log entry without duration
- ✅ Log entry without version

## Functions Tested

✅ Fully tested:
- `validatePullSecret()` - Pull secret validation
- `PullSecretTask.validateConfig()` - Configuration validation
- `PullSecretTask.TaskName()` - Task name retrieval
- `PullSecretJob.GetMetadata()` - Job metadata
- `PullSecretJob.GetWorkerCount()` - Worker count
- `PullSecretJob.GetTasks()` - Task creation
- `isRetryable()` - Retry logic
- `retryWithBackoff()` - Exponential backoff
- `logStructured()` - Structured logging

⚠️ Not tested (require GCP Secret Manager mock):
- `PullSecretTask.Process()` - Main processing logic
- `PullSecretTask.secretExists()` - Check if secret exists
- `PullSecretTask.createSecret()` - Create secret in GCP
- `PullSecretTask.addSecretVersion()` - Add secret version
- `PullSecretTask.verifySecret()` - Verify secret accessibility
- `PullSecretTask.createOrUpdateSecret()` - Create/update orchestration

## Running Tests

### Run all tests
```bash
make test
```

### Run tests with verbose output
```bash
go test -v ./cmd/pull-secret/jobs/
```

### Run specific test
```bash
go test -v -run TestValidatePullSecret ./cmd/pull-secret/jobs/
```

### Generate coverage report
```bash
make test
go tool cover -html=coverage.txt
```

### Run tests with race detector
```bash
go test -race ./cmd/pull-secret/jobs/
```

## Test Principles

1. **Table-Driven Tests**: All tests use table-driven approach for comprehensive coverage
2. **Isolation**: Tests use environment variable manipulation with proper cleanup
3. **Race Detection**: Tests run with `-race` flag to detect concurrency issues
4. **Coverage**: Target is 80%+ coverage for testable code
5. **Fast**: Unit tests complete in < 10 seconds

## Future Test Improvements

To increase coverage to 80%+, consider:

1. **Mock GCP Secret Manager Client**
   - Use interface abstraction for Secret Manager client
   - Create mock implementation for testing
   - Test Process(), secretExists(), createSecret(), etc.

2. **Integration Tests**
   - Test with real GCP Secret Manager (separate test suite)
   - Use test GCP project with cleanup
   - Validate end-to-end workflow

3. **Error Scenarios**
   - Test all GCP API error responses
   - Test network failures and timeouts
   - Test partial failures in retry logic

4. **Performance Tests**
   - Benchmark retry backoff timing
   - Measure memory allocation
   - Test concurrent task execution

## CI/CD Integration

Tests are automatically run in GitHub Actions on every pull request:
- Lint check
- Build check
- Test check with coverage report

See `.github/workflows/ci.yml` for CI configuration on GitHub.
