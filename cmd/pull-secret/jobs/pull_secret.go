package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/spf13/pflag"
	"gitlab.cee.redhat.com/service/hyperfleet/mvp/pkg/job"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	pullSecretTaskName = "pull-secret-mvp"

	// defaultPullSecretData is a fake pull secret used for testing when PULL_SECRET_DATA is not provided
	defaultPullSecretData = `{"auths":{"cloud.openshift.com":{"auth":"ZmFrZXVzZXI6ZmFrZXBhc3N3b3Jk","email":"user@example.com"},"quay.io":{"auth":"ZmFrZXVzZXI6ZmFrZXBhc3N3b3Jk","email":"user@example.com"},"registry.connect.redhat.com":{"auth":"ZmFrZXVzZXI6ZmFrZXBhc3N3b3Jk","email":"user@example.com"},"registry.redhat.io":{"auth":"ZmFrZXVzZXI6ZmFrZXBhc3N3b3Jk","email":"user@example.com"}}}`
)

type PullSecretTask struct {
	PullSecret   string
	GCPProjectID string
	ClusterID    string
	SecretName   string
	DryRun       bool
}

type PullSecretJob struct {
	DryRun bool
}

func (e PullSecretTask) TaskName() string {
	return pullSecretTaskName
}

func (pullsecretJob *PullSecretJob) GetTasks() ([]job.Task, error) {

	var tasks []job.Task

	// Read configuration from environment variables
	gcpProjectID := os.Getenv("GCP_PROJECT_ID")
	clusterID := os.Getenv("CLUSTER_ID")
	secretName := os.Getenv("SECRET_NAME")
	pullSecretData := os.Getenv("PULL_SECRET_DATA")

	// Use fake pull secret for testing if PULL_SECRET_DATA is not provided
	if pullSecretData == "" {
		pullSecretData = defaultPullSecretData
	}

	// Auto-derive secret name if not provided
	if secretName == "" && clusterID != "" {
		secretName = fmt.Sprintf("hyperfleet-%s-pull-secret", clusterID)
	}

	tasks = append(tasks, PullSecretTask{
		PullSecret:   pullSecretData,
		GCPProjectID: gcpProjectID,
		ClusterID:    clusterID,
		SecretName:   secretName,
		DryRun:       pullsecretJob.DryRun,
	})

	return tasks, nil
}

func (pullsecretJob *PullSecretJob) GetMetadata() job.Metadata {
	return job.Metadata{
		Use:         "pull-secret",
		Description: "Pull Secret Job Execution - Stores pull secret in GCP Secret Manager",
	}
}

func (pullsecretJob *PullSecretJob) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&pullsecretJob.DryRun, "dry-run", false, "Dry run mode - validate authentication and configuration without creating/updating secrets")
}

func (pullsecretJob *PullSecretJob) GetWorkerCount() int {
	return 1
}

func (e PullSecretTask) Process(ctx context.Context) error {

	// Validate required environment variables
	if err := e.validateConfig(); err != nil {
		logStructured("error", e.ClusterID, e.GCPProjectID, "validate-config", 0, err.Error(), "")
		return err
	}

	// Validate pull secret JSON format
	if err := validatePullSecret(e.PullSecret); err != nil {
		logStructured("error", e.ClusterID, e.GCPProjectID, "validate-pull-secret", 0, fmt.Sprintf("Invalid pull secret format: %v", err), "")
		return fmt.Errorf("invalid pull secret format: %w", err)
	}

	if e.DryRun {
		logStructured("info", e.ClusterID, e.GCPProjectID, "start", 0, "Starting pull secret storage operation (DRY RUN MODE)", "")
	} else {
		logStructured("info", e.ClusterID, e.GCPProjectID, "start", 0, "Starting pull secret storage operation", "")
	}

	// Initialize Secret Manager client
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		logStructured("error", e.ClusterID, e.GCPProjectID, "init-client", 0, fmt.Sprintf("Failed to create secretmanager client: %v", err), "")
		return fmt.Errorf("failed to create secretmanager client: %w", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			logStructured("error", e.ClusterID, e.GCPProjectID, "close-client", 0, fmt.Sprintf("Failed to close client: %v", closeErr), "")
		}
	}()

	logStructured("info", e.ClusterID, e.GCPProjectID, "client-initialized", 0, "Successfully initialized Secret Manager client", "")

	// In dry-run mode, skip actual secret operations
	if e.DryRun {
		logStructured("info", e.ClusterID, e.GCPProjectID, "dry-run", 0, "DRY RUN: Skipping secret creation/update operations", "")
		logStructured("info", e.ClusterID, e.GCPProjectID, "dry-run", 0, fmt.Sprintf("DRY RUN: Would create/update secret: %s", e.SecretName), "")
		logStructured("info", e.ClusterID, e.GCPProjectID, "completed", 0, "DRY RUN completed successfully - authentication validated", "")
		return nil
	}

	// Create or update secret with retry logic
	if err := retryWithBackoff(ctx, func() error {
		return e.createOrUpdateSecret(ctx, client)
	}, 3); err != nil {
		logStructured("error", e.ClusterID, e.GCPProjectID, "create-update-secret", 0, fmt.Sprintf("Failed to create/update secret: %v", err), "")
		return err
	}

	// Verify secret is accessible
	if err := retryWithBackoff(ctx, func() error {
		return e.verifySecret(ctx, client)
	}, 3); err != nil {
		logStructured("error", e.ClusterID, e.GCPProjectID, "verify-secret", 0, fmt.Sprintf("Failed to verify secret: %v", err), "")
		return err
	}

	logStructured("info", e.ClusterID, e.GCPProjectID, "completed", 0, "Successfully created/updated pull secret", "")

	return nil
}

// validateConfig validates required environment variables
func (e PullSecretTask) validateConfig() error {
	if e.GCPProjectID == "" {
		return fmt.Errorf("missing required environment variable: GCP_PROJECT_ID")
	}
	if e.ClusterID == "" {
		return fmt.Errorf("missing required environment variable: CLUSTER_ID")
	}
	if e.SecretName == "" {
		return fmt.Errorf("missing required environment variable: SECRET_NAME")
	}
	if e.PullSecret == "" {
		return fmt.Errorf("missing required environment variable: PULL_SECRET_DATA")
	}
	return nil
}

// createOrUpdateSecret creates or updates the secret in GCP Secret Manager
func (e PullSecretTask) createOrUpdateSecret(ctx context.Context, client *secretmanager.Client) error {
	startTime := time.Now()

	// Check if secret exists
	exists, err := e.secretExists(ctx, client)
	if err != nil {
		return err
	}

	if !exists {
		// Create new secret
		logStructured("info", e.ClusterID, e.GCPProjectID, "create-secret", 0, fmt.Sprintf("Creating new secret: %s", e.SecretName), "")
		if createErr := e.createSecret(ctx, client); createErr != nil {
			return fmt.Errorf("failed to create secret: %w", createErr)
		}
		duration := time.Since(startTime).Milliseconds()
		logStructured("info", e.ClusterID, e.GCPProjectID, "create-secret", duration, "Successfully created secret", "")
	} else {
		logStructured("info", e.ClusterID, e.GCPProjectID, "secret-exists", 0, fmt.Sprintf("Secret already exists: %s", e.SecretName), "")
	}

	// Add secret version with data
	startTime = time.Now()
	logStructured("info", e.ClusterID, e.GCPProjectID, "add-secret-version", 0, "Adding secret version with pull secret data", "")
	version, err := e.addSecretVersion(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to add secret version: %w", err)
	}
	duration := time.Since(startTime).Milliseconds()
	logStructured("info", e.ClusterID, e.GCPProjectID, "add-secret-version", duration, "Successfully created secret version", version)

	return nil
}

// secretExists checks if a secret exists in GCP Secret Manager
func (e PullSecretTask) secretExists(ctx context.Context, client *secretmanager.Client) (bool, error) {
	name := fmt.Sprintf("projects/%s/secrets/%s", e.GCPProjectID, e.SecretName)

	req := &secretmanagerpb.GetSecretRequest{
		Name: name,
	}

	_, err := client.GetSecret(ctx, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check secret existence: %w", err)
	}

	return true, nil
}

// createSecret creates a new secret in GCP Secret Manager
func (e PullSecretTask) createSecret(ctx context.Context, client *secretmanager.Client) error {
	req := &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", e.GCPProjectID),
		SecretId: e.SecretName,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
			Labels: map[string]string{
				"managed-by":         "hyperfleet",
				"adapter":            "pullsecret",
				"cluster-id":         e.ClusterID,
				"resource-type":      "pull-secret",
				"hyperfleet-version": "v1",
			},
		},
	}

	_, err := client.CreateSecret(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// addSecretVersion adds a new version with pull secret data
func (e PullSecretTask) addSecretVersion(ctx context.Context, client *secretmanager.Client) (string, error) {
	parent := fmt.Sprintf("projects/%s/secrets/%s", e.GCPProjectID, e.SecretName)

	req := &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(e.PullSecret),
		},
	}

	version, err := client.AddSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to add secret version: %w", err)
	}

	return version.Name, nil
}

// verifySecret verifies that the secret is accessible
func (e PullSecretTask) verifySecret(ctx context.Context, client *secretmanager.Client) error {
	startTime := time.Now()
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", e.GCPProjectID, e.SecretName)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to access secret version: %w", err)
	}

	duration := time.Since(startTime).Milliseconds()
	logStructured("info", e.ClusterID, e.GCPProjectID, "verify-secret", duration, fmt.Sprintf("Verified secret (%d bytes)", len(result.Payload.Data)), "")

	return nil
}

// validatePullSecret validates the pull secret JSON format
func validatePullSecret(pullSecretJSON string) error {
	var pullSecret map[string]interface{}
	if err := json.Unmarshal([]byte(pullSecretJSON), &pullSecret); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	auths, ok := pullSecret["auths"]
	if !ok {
		return fmt.Errorf("missing 'auths' key")
	}

	authsMap, ok := auths.(map[string]interface{})
	if !ok || len(authsMap) == 0 {
		return fmt.Errorf("'auths' must be a non-empty object")
	}

	return nil
}

// retryWithBackoff retries a function with exponential backoff
func retryWithBackoff(ctx context.Context, fn func() error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if !isRetryable(err) {
			return err
		}

		if i < maxRetries-1 {
			// Calculate backoff with jitter (±20%)
			baseBackoff := time.Duration(1<<uint(i)) * time.Second
			jitterRange := float64(baseBackoff) * 0.2
			// Random value between -20% and +20% of base backoff
			jitter := time.Duration((rand.Float64()*2 - 1) * jitterRange)
			backoff := baseBackoff + jitter

			log.Printf("Retry %d/%d after %s: %v", i+1, maxRetries, backoff, err)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return err
}

// isRetryable determines if an error is retryable
func isRetryable(err error) bool {
	code := status.Code(err)
	return code == codes.Unavailable ||
		code == codes.DeadlineExceeded ||
		code == codes.Internal ||
		code == codes.ResourceExhausted
}

// logStructured outputs structured JSON logs
func logStructured(level, clusterID, gcpProject, operation string, durationMs int64, message, version string) {
	logEntry := map[string]interface{}{
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"level":       level,
		"cluster_id":  clusterID,
		"gcp_project": gcpProject,
		"operation":   operation,
		"message":     message,
	}

	if durationMs > 0 {
		logEntry["duration_ms"] = durationMs
	}

	if version != "" {
		logEntry["version"] = version
	}

	jsonLog, err := json.Marshal(logEntry)
	if err != nil {
		log.Printf("Failed to marshal log entry: %v", err)
		return
	}

	fmt.Println(string(jsonLog))
}
