package config

import (
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
