package main

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// Test privilege detection in main command
func TestDetectPrivilegeLevelCmd(t *testing.T) {
	privLevel := detectPrivilegeLevelCmd()

	// Should contain platform-appropriate text
	switch runtime.GOOS {
	case "darwin", "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		if !strings.Contains(privLevel, "root") && !strings.Contains(privLevel, "sudo") {
			t.Errorf("Unix privilege level should contain 'root' or 'sudo', got: %s", privLevel)
		}
	case "windows":
		if !strings.Contains(privLevel, "administrator") && !strings.Contains(privLevel, "elevation") {
			t.Errorf("Windows privilege level should contain 'administrator' or 'elevation', got: %s", privLevel)
		}
	default:
		if privLevel != "unknown" {
			t.Errorf("Expected 'unknown' for unsupported platform, got: %s", privLevel)
		}
	}
}

// Test Windows admin detection in main command
func TestIsWindowsAdminCmd(t *testing.T) {
	isAdmin := isWindowsAdminCmd()

	if runtime.GOOS != "windows" {
		// On non-Windows systems, should always return false
		if isAdmin {
			t.Error("isWindowsAdminCmd should return false on non-Windows systems")
		}
	}
	// On Windows, the result depends on actual elevation status
	// We don't assert a specific value since it depends on test environment
	t.Logf("Windows admin detection result: %v", isAdmin)
}

// Test platform-specific sudo environment setup
func TestSetupPlatformSudoEnv(t *testing.T) {
	env := setupPlatformSudoEnv()

	// Should return environment variables
	if env == nil {
		t.Fatal("setupPlatformSudoEnv returned nil")
	}

	// Should contain current environment
	if len(env) == 0 {
		t.Error("setupPlatformSudoEnv should contain environment variables")
	}

	// Check for SUDO_ASKPASS on supported platforms
	sudoAskpassFound := false
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "SUDO_ASKPASS=") {
			sudoAskpassFound = true
			t.Logf("Found SUDO_ASKPASS: %s", envVar)

			// Validate that the path looks reasonable for the platform
			path := strings.TrimPrefix(envVar, "SUDO_ASKPASS=")
			switch runtime.GOOS {
			case "darwin":
				if !strings.Contains(path, "/opt/homebrew") && !strings.Contains(path, "/usr/local") {
					t.Logf("macOS askpass path may be non-standard: %s", path)
				}
			case "linux":
				if !strings.Contains(path, "/usr/bin") && !strings.Contains(path, "/usr/libexec") {
					t.Logf("Linux askpass path may be non-standard: %s", path)
				}
			}
			break
		}
	}

	t.Logf("SUDO_ASKPASS configured for platform %s: %v", runtime.GOOS, sudoAskpassFound)
}

// Test connectivity testing functions
func TestConnectivityHelpers(t *testing.T) {
	// Test ping connectivity to localhost (should work on all platforms)
	err := testPingConnectivity("127.0.0.1")
	if err != nil {
		t.Logf("Ping to localhost failed (may be expected in some test environments): %v", err)
	}

	// Test HTTP connectivity (this will fail, but we test the function structure)
	err = testHTTPConnectivity("127.0.0.1", 9999) // Non-existent port
	if err == nil {
		t.Error("HTTP connectivity to non-existent port should fail")
	}

	// Test SSH connectivity (this will fail, but we test the function structure)
	err = testSSHConnectivity("127.0.0.1", 9999) // Non-existent port
	if err == nil {
		t.Error("SSH connectivity to non-existent port should fail")
	}
}

// Test environment variable handling in different scenarios
func TestEnvironmentVariableHandling(t *testing.T) {
	// Save original values
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	origAppData := os.Getenv("APPDATA")

	defer func() {
		// Restore original values
		if origHome != "" {
			os.Setenv("HOME", origHome)
		} else {
			os.Unsetenv("HOME")
		}
		if origUserProfile != "" {
			os.Setenv("USERPROFILE", origUserProfile)
		} else {
			os.Unsetenv("USERPROFILE")
		}
		if origAppData != "" {
			os.Setenv("APPDATA", origAppData)
		} else {
			os.Unsetenv("APPDATA")
		}
	}()

	tests := []struct {
		name           string
		setHome        string
		setUserProfile string
		setAppData     string
		platform       string
	}{
		{
			name:     "Unix environment",
			setHome:  "/home/testuser",
			platform: "unix",
		},
		{
			name:           "Windows environment",
			setUserProfile: "C:\\Users\\testuser",
			setAppData:     "C:\\Users\\testuser\\AppData\\Roaming",
			platform:       "windows",
		},
		{
			name:     "Minimal environment",
			platform: "minimal",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear environment
			os.Unsetenv("HOME")
			os.Unsetenv("USERPROFILE")
			os.Unsetenv("APPDATA")

			// Set test environment
			if test.setHome != "" {
				os.Setenv("HOME", test.setHome)
			}
			if test.setUserProfile != "" {
				os.Setenv("USERPROFILE", test.setUserProfile)
			}
			if test.setAppData != "" {
				os.Setenv("APPDATA", test.setAppData)
			}

			// Test that functions handle the environment appropriately
			env := setupPlatformSudoEnv()
			if env == nil {
				t.Error("setupPlatformSudoEnv should handle any environment")
			}

			privLevel := detectPrivilegeLevelCmd()
			if privLevel == "" {
				t.Error("detectPrivilegeLevelCmd should always return something")
			}
		})
	}
}

// Test error handling for missing dependencies
func TestMissingDependencies(t *testing.T) {
	// Test behavior when commands are not available
	// (We can't easily test this without mocking, but we verify error handling)

	// The connectivity functions should handle missing commands gracefully
	err := testPingConnectivity("nonexistent.invalid.domain")
	if err == nil {
		t.Error("Ping to invalid domain should fail")
	}

	err = testHTTPConnectivity("nonexistent.invalid.domain", 80)
	if err == nil {
		t.Error("HTTP to invalid domain should fail")
	}

	err = testSSHConnectivity("nonexistent.invalid.domain", 22)
	if err == nil {
		t.Error("SSH to invalid domain should fail")
	}
}

// Test platform-specific cleanup functions
func TestCleanupFunctions(t *testing.T) {
	// We can't test actual WireGuard cleanup without root privileges
	// and real interfaces, but we can test the structure

	// Mock environment for testing
	env := []string{"PATH=/usr/bin:/bin", "HOME=/tmp"}

	// Test that cleanup functions handle empty interface lists
	interfaces := []string{}

	switch runtime.GOOS {
	case "darwin":
		err := cleanupMacOSInterfacesCmd(interfaces, env)
		if err != nil {
			t.Errorf("cleanupMacOSInterfacesCmd should handle empty interfaces: %v", err)
		}
	case "linux":
		err := cleanupLinuxInterfacesCmd(interfaces, env)
		if err != nil {
			t.Errorf("cleanupLinuxInterfacesCmd should handle empty interfaces: %v", err)
		}
	case "windows":
		err := cleanupWindowsInterfacesCmd(interfaces, env)
		if err != nil {
			t.Errorf("cleanupWindowsInterfacesCmd should handle empty interfaces: %v", err)
		}
	}

	// Test with whitespace-only interface names (should be filtered out)
	interfaces = []string{" ", "\t", "\n", ""}
	switch runtime.GOOS {
	case "darwin":
		err := cleanupMacOSInterfacesCmd(interfaces, env)
		if err != nil {
			t.Errorf("Cleanup should filter out empty/whitespace interfaces: %v", err)
		}
	case "linux":
		err := cleanupLinuxInterfacesCmd(interfaces, env)
		if err != nil {
			t.Errorf("Cleanup should filter out empty/whitespace interfaces: %v", err)
		}
	case "windows":
		err := cleanupWindowsInterfacesCmd(interfaces, env)
		if err != nil {
			t.Errorf("Cleanup should filter out empty/whitespace interfaces: %v", err)
		}
	}
}

// Test unsupported platform handling
func TestUnsupportedPlatformCleanup(t *testing.T) {
	// Test that unsupported platforms return appropriate errors
	// (We can't change runtime.GOOS, but we test the generic cleanup function)

	env := []string{"PATH=/usr/bin:/bin"}
	interfaces := []string{"test-interface"}

	err := cleanupGenericInterfacesCmd(interfaces, env)
	if err == nil {
		t.Error("cleanupGenericInterfacesCmd should return an error for unsupported platforms")
	}

	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("Error should mention unsupported platform, got: %v", err)
	}

	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("Error should mention current platform %s, got: %v", runtime.GOOS, err)
	}
}

// Test configuration cleanup function
func TestConfigCleanup(t *testing.T) {
	// Test cleanupLocalConfig with temporary directory
	origHome := os.Getenv("HOME")
	tempDir := "/tmp/mole-test-home"
	os.Setenv("HOME", tempDir)

	defer func() {
		os.Setenv("HOME", origHome)
		os.RemoveAll(tempDir)
	}()

	// Create test directories
	os.MkdirAll(tempDir+"/.mole/test", 0755)

	err := cleanupLocalConfig()
	if err != nil {
		t.Errorf("cleanupLocalConfig should handle cleanup gracefully: %v", err)
	}
}

// Benchmark privilege detection
func BenchmarkDetectPrivilegeLevelCmd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = detectPrivilegeLevelCmd()
	}
}

// Benchmark sudo environment setup
func BenchmarkSetupPlatformSudoEnv(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = setupPlatformSudoEnv()
	}
}

// Benchmark connectivity tests
func BenchmarkConnectivityTests(b *testing.B) {
	b.Run("ping", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testPingConnectivity("127.0.0.1")
		}
	})

	b.Run("http", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testHTTPConnectivity("127.0.0.1", 9999) // Will fail, but we're benchmarking
		}
	})

	b.Run("ssh", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testSSHConnectivity("127.0.0.1", 9999) // Will fail, but we're benchmarking
		}
	})
}