package config

import (
	"net"
	"os"
	"testing"
)

func FuzzConfigLoad(f *testing.F) {
	seeds := []string{
		`--my-call=QQ1ABC-1 --node-addr=127.0.0.1:1799`,
		`--http-addr=:8080 --log-level=debug`,
		`--data-dir=/tmp/meshcom --max-message-length=100`,
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, argsStr string) {
		// Mock args
		os.Args = append([]string{"cmd"}, splitArgs(argsStr)...)

		// Use a temporary data dir to avoid side effects
		os.Setenv("GOMESHCOM_DATA_DIR", t.TempDir())

		_, _, _ = Load("fuzz")
	})
}

func splitArgs(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s} // Simplistic, but enough for basic fuzzing of flags
}

func FuzzParseForwardTargets(f *testing.F) {
	seeds := []string{
		"",
		"127.0.0.1:9000",
		"127.0.0.1:9000,127.0.0.1:9001",
		" 10.0.0.1:1799 , 10.0.0.2:1799 ",
		"bad:notaport",
		",,",
		"127.0.0.1:9000,127.0.0.1:9000",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, csv string) {
		// Must never panic. Errors are acceptable.
		targets, err := ParseForwardTargets(csv)
		if err != nil {
			return
		}
		// All returned targets must be resolvable.
		for _, target := range targets {
			if _, resolveErr := net.ResolveUDPAddr("udp", target); resolveErr != nil {
				t.Fatalf("ParseForwardTargets returned invalid target %q: %v", target, resolveErr)
			}
		}
		// No duplicates.
		seen := make(map[string]bool, len(targets))
		for _, target := range targets {
			if seen[target] {
				t.Fatalf("ParseForwardTargets returned duplicate target %q", target)
			}
			seen[target] = true
		}
	})
}
