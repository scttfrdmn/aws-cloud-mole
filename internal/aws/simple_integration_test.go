package aws

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// TestLocalStackIntegration provides a simple integration test with LocalStack
func TestLocalStackIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LocalStack integration tests in short mode")
	}

	// Check if LocalStack is available or try to start it
	if !isLocalStackRunning() {
		if !startLocalStackSimple(t) {
			t.Skip("LocalStack not available and could not start it")
		}
		defer stopLocalStackSimple(t)
		waitForLocalStackSimple(t)
	}

	t.Run("basic LocalStack connectivity", func(t *testing.T) {
		client := createSimpleLocalStackClient(t)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Test basic EC2 connectivity
		_, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
		if err != nil {
			t.Fatalf("Failed to connect to LocalStack EC2: %v", err)
		}

		t.Log("Successfully connected to LocalStack EC2")
	})

	t.Run("LocalStack vs Mock strategy", func(t *testing.T) {
		// Demonstrate when to use LocalStack vs mocks
		t.Log("Use LocalStack for:")
		t.Log("  - Integration tests with real AWS APIs")
		t.Log("  - End-to-end workflow testing")
		t.Log("  - AWS service interaction validation")

		t.Log("Use mocks for:")
		t.Log("  - Fast unit tests")
		t.Log("  - Business logic validation")
		t.Log("  - Error condition testing")
		t.Log("  - CI/CD pipeline tests")

		// Test mock for speed
		mock := NewMockAWSClient()
		_, err := mock.CreateBastion(context.Background(), &BastionConfig{})
		if err != nil {
			t.Errorf("Mock should work without external dependencies: %v", err)
		}

		// Test LocalStack for real API interaction
		if isLocalStackRunning() {
			client := createSimpleLocalStackClient(t)
			_, err := client.DescribeRegions(context.Background(), &ec2.DescribeRegionsInput{})
			if err != nil {
				t.Errorf("LocalStack should provide real AWS API: %v", err)
			}
		}
	})
}

// isLocalStackRunning checks if LocalStack is already running
func isLocalStackRunning() bool {
	client := createSimpleLocalStackClientUnsafe()
	if client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{})
	return err == nil
}

// startLocalStackSimple starts LocalStack with Podman
func startLocalStackSimple(t *testing.T) bool {
	t.Log("Attempting to start LocalStack with Podman...")

	// Check if podman is available
	if _, err := exec.LookPath("podman"); err != nil {
		t.Log("Podman not found in PATH")
		return false
	}

	// Start LocalStack container
	cmd := exec.Command("podman", "run", "--rm", "-d",
		"--name", "localstack-mole-test",
		"-p", "4566:4566",
		"-e", "SERVICES=ec2",
		"-e", "DEBUG=0",
		"localstack/localstack:latest")

	if err := cmd.Run(); err != nil {
		t.Logf("Failed to start LocalStack: %v", err)
		return false
	}

	t.Log("LocalStack container started")
	return true
}

// stopLocalStackSimple stops the LocalStack container
func stopLocalStackSimple(t *testing.T) {
	t.Log("Stopping LocalStack container...")
	cmd := exec.Command("podman", "stop", "localstack-mole-test")
	_ = cmd.Run() // Ignore errors on cleanup
}

// waitForLocalStackSimple waits for LocalStack to be ready
func waitForLocalStackSimple(t *testing.T) {
	t.Log("Waiting for LocalStack to be ready...")

	for i := 0; i < 20; i++ {
		if isLocalStackRunning() {
			t.Log("LocalStack is ready")
			return
		}
		time.Sleep(3 * time.Second)
	}

	t.Fatal("LocalStack did not become ready in time")
}

// createSimpleLocalStackClient creates a LocalStack EC2 client with error handling
func createSimpleLocalStackClient(t *testing.T) *ec2.Client {
	client := createSimpleLocalStackClientUnsafe()
	if client == nil {
		t.Fatal("Failed to create LocalStack client")
	}
	return client
}

// createSimpleLocalStackClientUnsafe creates a LocalStack client without error handling
func createSimpleLocalStackClientUnsafe() *ec2.Client {
	// Set LocalStack credentials
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:           "http://localhost:4566",
					SigningRegion: "us-east-1",
				}, nil
			},
		)),
	)
	if err != nil {
		return nil
	}

	return ec2.NewFromConfig(cfg)
}

// TestAWSClientWithLocalStack tests the actual AWSClient against LocalStack
func TestAWSClientWithLocalStack(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LocalStack tests in short mode")
	}

	if !isLocalStackRunning() {
		t.Skip("LocalStack not running - start with: podman run -p 4566:4566 localstack/localstack")
	}

	// Set LocalStack environment
	original := setupLocalStackEnv()
	defer restoreEnv(original)

	// Test AWSClient creation with LocalStack endpoint
	client, err := NewAWSClient("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Failed to create AWSClient for LocalStack: %v", err)
	}

	if client == nil {
		t.Fatal("AWSClient should not be nil")
	}

	// Test placeholder methods (these don't hit real AWS APIs yet)
	t.Run("placeholder methods with LocalStack environment", func(t *testing.T) {
		ctx := context.Background()

		// Test CreateBastion (placeholder implementation)
		info, err := client.CreateBastion(ctx, &BastionConfig{})
		if err != nil {
			t.Errorf("CreateBastion placeholder should work: %v", err)
		}
		if info == nil {
			t.Error("CreateBastion should return info")
		}

		// Test CreateSecurityGroups (placeholder implementation)
		sgID, err := client.CreateSecurityGroups(ctx, "vpc-test", 2)
		if err != nil {
			t.Errorf("CreateSecurityGroups placeholder should work: %v", err)
		}
		if sgID == "" {
			t.Error("CreateSecurityGroups should return ID")
		}

		t.Log("Placeholder methods tested successfully")
	})
}

// setupLocalStackEnv sets LocalStack environment variables
func setupLocalStackEnv() map[string]string {
	original := make(map[string]string)

	envVars := []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_ENDPOINT_URL"}
	for _, key := range envVars {
		original[key] = os.Getenv(key)
	}

	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")

	return original
}

// restoreEnv restores original environment variables
func restoreEnv(original map[string]string) {
	for key, value := range original {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}