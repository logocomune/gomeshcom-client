// Package apprestart provides helpers for detecting the runtime environment
// (container vs. standalone) and for re-executing the current binary in-place.
package apprestart

import (
	"os"
	"os/exec"
	"strings"
)

// RunningInContainer reports whether the process is running inside a container
// (Docker, containerd, Kubernetes, …).
//
// Detection order:
//  1. RUNNING_IN_DOCKER env var (explicit config wins over autodetect):
//     "1" → true, any other non-empty value → false.
//  2. Presence of /.dockerenv (Linux only, Docker legacy marker).
//  3. /proc/1/cgroup content containing "docker", "containerd", or "kubepods"
//     (Linux cgroup v1; cgroup v2 may give false negatives, hence the env var
//     is the preferred signal).
func RunningInContainer() bool {
	if v, ok := os.LookupEnv("RUNNING_IN_DOCKER"); ok {
		return v == "1"
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		s := string(data)
		if strings.Contains(s, "docker") ||
			strings.Contains(s, "containerd") ||
			strings.Contains(s, "kubepods") {
			return true
		}
	}

	return false
}

// RestartSelf launches a new instance of the current executable with the same
// arguments, environment, and working directory, then returns so the caller
// can exit the current process.
//
// It uses cmd.Start (not Wait) so the child outlives the parent.
func RestartSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// When running via `go run` or a hot-reload tool (e.g. air), the compiled
	// binary is written to a temp path that may be deleted before RestartSelf
	// is called. /proc/self/exe is a Linux kernel symlink to the live inode
	// and remains valid even after the original path is unlinked.
	if _, statErr := os.Stat(exe); statErr != nil {
		if _, procErr := os.Stat("/proc/self/exe"); procErr == nil {
			exe = "/proc/self/exe"
		}
	}

	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	if dir, err := os.Getwd(); err == nil {
		cmd.Dir = dir
	}

	return cmd.Start()
}
