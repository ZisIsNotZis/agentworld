package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFoodFlowCLIRejectsPositionalArgumentsWithoutPublication(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "foodflow")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, out)
	}
	for _, tc := range []struct {
		name string
		args func(string) []string
	}{
		{"before-flags", func(path string) []string { return []string{"stray", "-q=0", "-checkpoint=" + path} }},
		{"after-flags", func(path string) []string { return []string{"-q=0", "-checkpoint=" + path, "stray"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "must-not-exist.bundle")
			cmd := exec.CommandContext(ctx, binary, tc.args(path)...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() == 0 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("unexpected positional arguments")) {
				t.Fatalf("accepted positional arguments: status=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("checkpoint published on invalid input: %v", err)
			}
		})
	}
}
