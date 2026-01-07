package jobs

import (
	"context"
	"os"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestValidatePullSecret tests the pull secret validation function
func TestValidatePullSecret(t *testing.T) {
	tests := []struct {
		name        string
		pullSecret  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid pull secret",
			pullSecret: `{
				"auths": {
					"registry.redhat.io": {
						"auth": "dGVzdDp0ZXN0",
						"email": "test@example.com"
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "valid pull secret with multiple registries",
			pullSecret: `{
				"auths": {
					"registry.redhat.io": {
						"auth": "dGVzdDp0ZXN0",
						"email": "test@example.com"
					},
					"quay.io": {
						"auth": "dGVzdDp0ZXN0",
						"email": "test@example.com"
					}
				}
			}`,
			expectError: false,
		},
		{
			name:        "invalid JSON",
			pullSecret:  `{invalid json`,
			expectError: true,
			errorMsg:    "invalid JSON",
		},
		{
			name:        "missing auths key",
			pullSecret:  `{"registries": {}}`,
			expectError: true,
			errorMsg:    "missing 'auths' key",
		},
		{
			name:        "empty auths object",
			pullSecret:  `{"auths": {}}`,
			expectError: true,
			errorMsg:    "'auths' must be a non-empty object",
		},
		{
			name:        "auths is not an object",
			pullSecret:  `{"auths": "string"}`,
			expectError: true,
			errorMsg:    "'auths' must be a non-empty object",
		},
		{
			name:        "empty pull secret",
			pullSecret:  ``,
			expectError: true,
			errorMsg:    "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePullSecret(tt.pullSecret)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', got nil", tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestPullSecretTask_validateConfig tests configuration validation
func TestPullSecretTask_validateConfig(t *testing.T) {
	tests := []struct {
		name        string
		task        PullSecretTask
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
			task: PullSecretTask{
				GCPProjectID: "test-project",
				ClusterID:    "cls-123",
				SecretName:   "test-secret",
				PullSecret:   "test-data",
			},
			expectError: false,
		},
		{
			name: "missing GCP project ID",
			task: PullSecretTask{
				ClusterID:  "cls-123",
				SecretName: "test-secret",
				PullSecret: "test-data",
			},
			expectError: true,
			errorMsg:    "GCP_PROJECT_ID",
		},
		{
			name: "missing cluster ID",
			task: PullSecretTask{
				GCPProjectID: "test-project",
				SecretName:   "test-secret",
				PullSecret:   "test-data",
			},
			expectError: true,
			errorMsg:    "CLUSTER_ID",
		},
		{
			name: "missing secret name",
			task: PullSecretTask{
				GCPProjectID: "test-project",
				ClusterID:    "cls-123",
				PullSecret:   "test-data",
			},
			expectError: true,
			errorMsg:    "SECRET_NAME",
		},
		{
			name: "missing pull secret data",
			task: PullSecretTask{
				GCPProjectID: "test-project",
				ClusterID:    "cls-123",
				SecretName:   "test-secret",
			},
			expectError: true,
			errorMsg:    "PULL_SECRET_DATA",
		},
		{
			name:        "all fields empty",
			task:        PullSecretTask{},
			expectError: true,
			errorMsg:    "GCP_PROJECT_ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.task.validateConfig()
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestPullSecretTask_TaskName tests the TaskName method
func TestPullSecretTask_TaskName(t *testing.T) {
	task := PullSecretTask{}
	expected := "pull-secret-mvp"
	got := task.TaskName()

	if got != expected {
		t.Errorf("expected task name '%s', got '%s'", expected, got)
	}
}

// TestPullSecretJob_GetMetadata tests the GetMetadata method
func TestPullSecretJob_GetMetadata(t *testing.T) {
	job := &PullSecretJob{}
	metadata := job.GetMetadata()

	if metadata.Use != "pull-secret" {
		t.Errorf("expected Use 'pull-secret', got '%s'", metadata.Use)
	}

	if metadata.Description == "" {
		t.Error("expected non-empty Description")
	}
}

// TestPullSecretJob_GetWorkerCount tests the GetWorkerCount method
func TestPullSecretJob_GetWorkerCount(t *testing.T) {
	job := &PullSecretJob{}
	expected := 1
	got := job.GetWorkerCount()

	if got != expected {
		t.Errorf("expected worker count %d, got %d", expected, got)
	}
}

// TestPullSecretJob_GetTasks tests the GetTasks method with environment variables
func TestPullSecretJob_GetTasks(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		checkTask   func(*testing.T, PullSecretTask)
	}{
		{
			name: "all environment variables set",
			envVars: map[string]string{
				"GCP_PROJECT_ID":   "test-project",
				"CLUSTER_ID":       "cls-123",
				"SECRET_NAME":      "custom-secret",
				"PULL_SECRET_DATA": `{"auths":{"registry.io":{"auth":"dGVzdA=="}}}`,
			},
			expectError: false,
			checkTask: func(t *testing.T, task PullSecretTask) {
				if task.GCPProjectID != "test-project" {
					t.Errorf("expected GCPProjectID 'test-project', got '%s'", task.GCPProjectID)
				}
				if task.ClusterID != "cls-123" {
					t.Errorf("expected ClusterID 'cls-123', got '%s'", task.ClusterID)
				}
				if task.SecretName != "custom-secret" {
					t.Errorf("expected SecretName 'custom-secret', got '%s'", task.SecretName)
				}
			},
		},
		{
			name: "auto-generate secret name",
			envVars: map[string]string{
				"GCP_PROJECT_ID":   "test-project",
				"CLUSTER_ID":       "cls-456",
				"PULL_SECRET_DATA": `{"auths":{"registry.io":{"auth":"dGVzdA=="}}}`,
			},
			expectError: false,
			checkTask: func(t *testing.T, task PullSecretTask) {
				expected := "hyperfleet-cls-456-pull-secret"
				if task.SecretName != expected {
					t.Errorf("expected auto-generated SecretName '%s', got '%s'", expected, task.SecretName)
				}
			},
		},
		{
			name: "missing PULL_SECRET_DATA returns error",
			envVars: map[string]string{
				"GCP_PROJECT_ID": "test-project",
				"CLUSTER_ID":     "cls-789",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set environment variables
			os.Clearenv()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer os.Clearenv()

			job := &PullSecretJob{}
			tasks, err := job.GetTasks()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("expected no error, got: %v", err)
				return
			}

			if len(tasks) != 1 {
				t.Errorf("expected 1 task, got %d", len(tasks))
				return
			}

			task, ok := tasks[0].(PullSecretTask)
			if !ok {
				t.Error("expected task to be PullSecretTask")
				return
			}

			if tt.checkTask != nil {
				tt.checkTask(t, task)
			}
		})
	}
}

// TestIsRetryable tests the retry logic for different error codes
func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable - Unavailable",
			err:      status.Error(codes.Unavailable, "service unavailable"),
			expected: true,
		},
		{
			name:     "retryable - DeadlineExceeded",
			err:      status.Error(codes.DeadlineExceeded, "deadline exceeded"),
			expected: true,
		},
		{
			name:     "retryable - Internal",
			err:      status.Error(codes.Internal, "internal error"),
			expected: true,
		},
		{
			name:     "retryable - ResourceExhausted",
			err:      status.Error(codes.ResourceExhausted, "rate limit exceeded"),
			expected: true,
		},
		{
			name:     "not retryable - PermissionDenied",
			err:      status.Error(codes.PermissionDenied, "permission denied"),
			expected: false,
		},
		{
			name:     "not retryable - NotFound",
			err:      status.Error(codes.NotFound, "not found"),
			expected: false,
		},
		{
			name:     "not retryable - AlreadyExists",
			err:      status.Error(codes.AlreadyExists, "already exists"),
			expected: false,
		},
		{
			name:     "not retryable - InvalidArgument",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("expected isRetryable(%v) = %v, got %v", tt.err, tt.expected, result)
			}
		})
	}
}

// TestRetryWithBackoff tests the retry mechanism
func TestRetryWithBackoff(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		attempts := 0
		fn := func() error {
			attempts++
			return nil
		}

		ctx := context.Background()
		err := retryWithBackoff(ctx, fn, 3)

		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		fn := func() error {
			attempts++
			if attempts < 3 {
				return status.Error(codes.Unavailable, "unavailable")
			}
			return nil
		}

		ctx := context.Background()
		err := retryWithBackoff(ctx, fn, 5)

		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}

		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		fn := func() error {
			attempts++
			return status.Error(codes.Unavailable, "unavailable")
		}

		ctx := context.Background()
		err := retryWithBackoff(ctx, fn, 3)

		if err == nil {
			t.Error("expected error, got nil")
		}

		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error stops immediately", func(t *testing.T) {
		attempts := 0
		fn := func() error {
			attempts++
			return status.Error(codes.PermissionDenied, "permission denied")
		}

		ctx := context.Background()
		err := retryWithBackoff(ctx, fn, 3)

		if err == nil {
			t.Error("expected error, got nil")
		}

		if attempts != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		attempts := 0
		fn := func() error {
			attempts++
			return status.Error(codes.Unavailable, "unavailable")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := retryWithBackoff(ctx, fn, 3)

		if err != context.Canceled {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
	})
}

// TestLogStructured verifies that logStructured doesn't panic
func TestLogStructured(t *testing.T) {
	tests := []struct {
		name       string
		level      string
		clusterID  string
		gcpProject string
		operation  string
		durationMs int64
		message    string
		version    string
	}{
		{
			name:       "complete log entry",
			level:      "info",
			clusterID:  "cls-123",
			gcpProject: "test-project",
			operation:  "test-operation",
			durationMs: 100,
			message:    "test message",
			version:    "v1",
		},
		{
			name:       "log entry without duration",
			level:      "error",
			clusterID:  "cls-456",
			gcpProject: "test-project",
			operation:  "test-operation",
			durationMs: 0,
			message:    "error message",
			version:    "",
		},
		{
			name:       "log entry without version",
			level:      "info",
			clusterID:  "cls-789",
			gcpProject: "test-project",
			operation:  "test-operation",
			durationMs: 200,
			message:    "test message",
			version:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("logStructured panicked: %v", r)
				}
			}()

			logStructured(tt.level, tt.clusterID, tt.gcpProject, tt.operation, tt.durationMs, tt.message, tt.version)
		})
	}
}
