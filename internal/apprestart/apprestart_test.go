package apprestart_test

import (
	"os"
	"testing"

	"github.com/logocomune/gomeshcom-client/internal/apprestart"
)

// TestMain intercepts child-process executions before the testing framework
// wraps os.Exit. When APPRESTART_TEST_CHILD=1 the process exits immediately
// without running any tests, acting as a controlled target for RestartSelf.
func TestMain(m *testing.M) {
	if os.Getenv("APPRESTART_TEST_CHILD") == "1" {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestRunningInContainerEnvOverride verifies that the RUNNING_IN_DOCKER env var
// is the primary signal and overrides any file-based autodetection.
func TestRunningInContainerEnvOverride(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		envSet bool
		want   bool
	}{
		{name: "env=1 means container", envVal: "1", envSet: true, want: true},
		{name: "env=0 means not container", envVal: "0", envSet: true, want: false},
		{name: "env=true means not container (only '1' is recognized)", envVal: "true", envSet: true, want: false},
		// When unset, the function falls through to file-based detection.
		// On the CI host (not a container) this should return false, but we
		// only assert the env-based cases here to keep the test deterministic.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSet {
				t.Setenv("RUNNING_IN_DOCKER", tt.envVal)
			} else {
				os.Unsetenv("RUNNING_IN_DOCKER")
			}

			got := apprestart.RunningInContainer()
			if got != tt.want {
				t.Errorf("RunningInContainer() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRestartSelfSentinel verifies that RestartSelf can spawn the child process
// without error. The child binary detects APPRESTART_TEST_CHILD=1 in TestMain
// and exits immediately, preventing a fork bomb.
func TestRestartSelfSentinel(t *testing.T) {
	t.Setenv("APPRESTART_TEST_CHILD", "1")

	if err := apprestart.RestartSelf(); err != nil {
		t.Fatalf("RestartSelf() error = %v", err)
	}
	// Success: child process started. It will exit on its own via TestMain.
}
