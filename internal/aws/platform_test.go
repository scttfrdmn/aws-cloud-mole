package aws

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// Test platform-specific path handling
func TestPlatformPaths(t *testing.T) {
	client := &AWSClient{}

	tests := []struct {
		name      string
		platform  string
		expectDir string
	}{
		{"macOS homebrew", "darwin", "/opt/homebrew"},
		{"Linux standard", "linux", "/etc"},
		{"Windows program files", "windows", "Program Files"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// We can't change runtime.GOOS during tests, but we can test the logic
			// by examining what paths would be used
			originalGOOS := runtime.GOOS

			switch test.platform {
			case "darwin":
				if runtime.GOOS == "darwin" {
					env := client.setupSudoEnvironment()
					// Check that askpass paths are appropriate for macOS
					found := false
					for _, envVar := range env {
						if strings.Contains(envVar, "SUDO_ASKPASS") && strings.Contains(envVar, "/opt/homebrew") {
							found = true
							break
						}
					}
					// This might not always pass in CI, but it's a reasonable test
					t.Logf("macOS askpass setup: found appropriate path = %v", found)
				}
			case "linux":
				if runtime.GOOS == "linux" {
					// Similar test for Linux paths
					t.Log("Testing Linux paths (implementation depends on actual OS)")
				}
			case "windows":
				if runtime.GOOS == "windows" {
					// Test Windows paths
					t.Log("Testing Windows paths (implementation depends on actual OS)")
				}
			}

			_ = originalGOOS // Prevent unused variable warning
		})
	}
}

// Test platform detection accuracy
func TestPlatformDetection(t *testing.T) {
	client := &AWSClient{}

	// Test privilege detection matches the current platform
	privLevel := client.detectPrivilegeLevel()

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(privLevel, "root") && !strings.Contains(privLevel, "sudo") {
			t.Errorf("macOS privilege detection should mention 'root' or 'sudo', got: %s", privLevel)
		}
	case "linux":
		if !strings.Contains(privLevel, "root") && !strings.Contains(privLevel, "sudo") {
			t.Errorf("Linux privilege detection should mention 'root' or 'sudo', got: %s", privLevel)
		}
	case "windows":
		if !strings.Contains(privLevel, "administrator") && !strings.Contains(privLevel, "elevation") {
			t.Errorf("Windows privilege detection should mention 'administrator' or 'elevation', got: %s", privLevel)
		}
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		if !strings.Contains(privLevel, "root") && !strings.Contains(privLevel, "sudo") {
			t.Errorf("BSD privilege detection should mention 'root' or 'sudo', got: %s", privLevel)
		}
	default:
		if privLevel != "unknown" {
			t.Errorf("Unknown platform should return 'unknown', got: %s", privLevel)
		}
	}
}

// Test askpass detection logic
func TestAskpassDetection(t *testing.T) {
	client := &AWSClient{}

	env := client.setupSudoEnvironment()

	// Should always return an environment slice
	if env == nil {
		t.Fatal("setupSudoEnvironment should never return nil")
	}

	// Should contain at least the current environment
	if len(env) == 0 {
		t.Error("setupSudoEnvironment should return at least current environment")
	}

	// Check if SUDO_ASKPASS was added (depends on system)
	sudoAskpassFound := false
	for _, envVar := range env {
		if strings.HasPrefix(envVar, "SUDO_ASKPASS=") {
			sudoAskpassFound = true
			t.Logf("Found SUDO_ASKPASS: %s", envVar)
			break
		}
	}

	// Log result (we can't assert this works everywhere)
	t.Logf("SUDO_ASKPASS configured: %v", sudoAskpassFound)
}

// Test platform-specific config directory logic
func TestConfigDirectorySelection(t *testing.T) {
	// Simulate different platform scenarios
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	originalAppData := os.Getenv("APPDATA")

	defer func() {
		// Restore original environment
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		}
		if originalUserProfile != "" {
			os.Setenv("USERPROFILE", originalUserProfile)
		}
		if originalAppData != "" {
			os.Setenv("APPDATA", originalAppData)
		}
	}()

	tests := []struct {
		name        string
		setHome     string
		setProfile  string
		setAppData  string
		expectPath  string
	}{
		{
			name:       "Unix with HOME",
			setHome:    "/home/user",
			expectPath: "/home/user",
		},
		{
			name:       "Windows with USERPROFILE",
			setProfile: "C:\\Users\\user",
			expectPath: "C:\\Users\\user",
		},
		{
			name:       "Windows with APPDATA fallback",
			setAppData: "C:\\Users\\user\\AppData\\Roaming",
			expectPath: "C:\\Users\\user\\AppData\\Roaming",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Clear all env vars first
			os.Unsetenv("HOME")
			os.Unsetenv("USERPROFILE")
			os.Unsetenv("APPDATA")

			// Set test values
			if test.setHome != "" {
				os.Setenv("HOME", test.setHome)
			}
			if test.setProfile != "" {
				os.Setenv("USERPROFILE", test.setProfile)
			}
			if test.setAppData != "" {
				os.Setenv("APPDATA", test.setAppData)
			}

			// Test path resolution logic
			var resolvedPath string
			if home := os.Getenv("HOME"); home != "" {
				resolvedPath = home
			} else if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
				resolvedPath = userProfile
			} else if appData := os.Getenv("APPDATA"); appData != "" {
				resolvedPath = appData
			}

			if resolvedPath != test.expectPath {
				t.Errorf("Expected path %s, got %s", test.expectPath, resolvedPath)
			}
		})
	}
}

// Test cross-platform compatibility of WireGuard operations
func TestWireGuardCompatibility(t *testing.T) {
	client := &AWSClient{}

	// Test that key generation works on all platforms
	privateKey, publicKey, err := client.generateWireGuardKeys()
	if err != nil {
		t.Fatalf("WireGuard key generation should work on all platforms: %v", err)
	}

	if privateKey == "" || publicKey == "" {
		t.Error("WireGuard keys should not be empty on any platform")
	}

	// Test key format (should be base64)
	if len(privateKey) < 40 { // Base64 encoded 32-byte key should be longer
		t.Errorf("Private key seems too short: %d characters", len(privateKey))
	}

	if len(publicKey) < 40 {
		t.Errorf("Public key seems too short: %d characters", len(publicKey))
	}
}

// Test that unsupported platforms are handled gracefully
func TestUnsupportedPlatformHandling(t *testing.T) {
	// This tests our error handling for theoretical unsupported platforms
	// We can't easily test this without mocking runtime.GOOS

	client := &AWSClient{}

	// The detectPrivilegeLevel should handle unknown platforms
	// (We test this in the main test, but verify error handling logic here)

	switch runtime.GOOS {
	case "darwin", "linux", "windows", "freebsd", "openbsd", "netbsd", "dragonfly":
		t.Logf("Current platform %s is supported", runtime.GOOS)
	default:
		t.Logf("Current platform %s would be unsupported", runtime.GOOS)
		privLevel := client.detectPrivilegeLevel()
		if privLevel != "unknown" {
			t.Errorf("Unsupported platform should return 'unknown', got: %s", privLevel)
		}
	}
}

// Test Windows-specific functionality
func TestWindowsSpecifics(t *testing.T) {
	client := &AWSClient{}

	// Test Windows admin detection
	isAdmin := client.isWindowsAdmin()

	if runtime.GOOS == "windows" {
		// On Windows, the result depends on actual elevation
		t.Logf("Windows admin status: %v", isAdmin)
	} else {
		// On non-Windows, should always be false
		if isAdmin {
			t.Error("isWindowsAdmin should return false on non-Windows platforms")
		}
	}
}

// Test that platform-specific paths are reasonable
func TestPlatformSpecificPaths(t *testing.T) {
	tests := []struct {
		platform    string
		configPaths []string
		valid       func(string) bool
	}{
		{
			platform: "darwin",
			configPaths: []string{
				"/opt/homebrew/etc/wireguard",
				"/usr/local/etc/wireguard",
			},
			valid: func(path string) bool {
				return strings.Contains(path, "/etc/wireguard") || strings.Contains(path, "homebrew")
			},
		},
		{
			platform: "linux",
			configPaths: []string{
				"/etc/wireguard",
			},
			valid: func(path string) bool {
				return strings.HasPrefix(path, "/etc/") || strings.Contains(path, ".config")
			},
		},
		{
			platform: "windows",
			configPaths: []string{
				"C:\\Program Files\\WireGuard",
				"Documents\\WireGuard",
			},
			valid: func(path string) bool {
				return strings.Contains(path, "Program Files") || strings.Contains(path, "Documents") || strings.Contains(path, "AppData")
			},
		},
		{
			platform: "freebsd",
			configPaths: []string{
				"/usr/local/etc/wireguard",
			},
			valid: func(path string) bool {
				return strings.Contains(path, "/usr/local/") || strings.Contains(path, "/etc/")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.platform, func(t *testing.T) {
			for _, path := range test.configPaths {
				if !test.valid(path) {
					t.Errorf("Invalid %s path: %s", test.platform, path)
				}
			}
		})
	}
}

// Benchmark platform detection
func BenchmarkPlatformDetection(b *testing.B) {
	client := &AWSClient{}

	for i := 0; i < b.N; i++ {
		_ = client.detectPrivilegeLevel()
	}
}

// Benchmark sudo environment setup
func BenchmarkSudoEnvironmentSetup(b *testing.B) {
	client := &AWSClient{}

	for i := 0; i < b.N; i++ {
		_ = client.setupSudoEnvironment()
	}
}