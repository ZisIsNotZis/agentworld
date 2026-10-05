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

func buildNaturalCLI(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "natural")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, out)
	}
	return binary
}

// TestNaturalCLIRejectsPositionalArgumentsWithoutPublication pins the stray
// argument contract: exit status 2 before any publication, with the output
// file absent and the checkpoint directory never created.
func TestNaturalCLIRejectsPositionalArgumentsWithoutPublication(t *testing.T) {
	binary := buildNaturalCLI(t)
	for _, tc := range []struct {
		name string
		args func(out, dir string) []string
	}{
		{"before-flags", func(out, dir string) []string {
			return []string{"stray", "-r=8", "-hours=1", "-out=" + out, "-checkpoint-dir=" + dir}
		}},
		{"after-flags", func(out, dir string) []string {
			return []string{"-r=8", "-hours=1", "-out=" + out, "-checkpoint-dir=" + dir, "stray"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			out := filepath.Join(base, "report.json")
			dir := filepath.Join(base, "bundles")
			cmd := exec.CommandContext(context.Background(), binary, tc.args(out, dir)...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 {
				t.Fatalf("stray args exit status: %v (stderr %s)", err, stderr.String())
			}
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				t.Fatalf("output published despite stray args: %v", err)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("checkpoint directory created despite stray args: %v", err)
			}
		})
	}
}
