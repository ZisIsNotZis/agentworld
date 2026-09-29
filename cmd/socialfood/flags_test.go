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

func buildSocialFoodCLI(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "socialfood")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, out)
	}
	return binary
}

func TestSocialFoodCLIRejectsPositionalArgumentsWithoutPublication(t *testing.T) {
	binary := buildSocialFoodCLI(t)
	for _, tc := range []struct {
		name string
		args func(string, string) []string
	}{
		{"before-flags", func(out, dir string) []string {
			return []string{"stray", "-q=3", "-hours=1", "-out=" + out, "-checkpoint-dir=" + dir}
		}},
		{"after-flags", func(out, dir string) []string {
			return []string{"-q=3", "-hours=1", "-out=" + out, "-checkpoint-dir=" + dir, "stray"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			out := filepath.Join(base, "report.json")
			dir := filepath.Join(base, "bundles")
			cmd := exec.Command(binary, tc.args(out, dir)...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || stdout.Len() != 0 || !bytes.Contains(stderr.Bytes(), []byte("unexpected positional arguments")) {
				t.Fatalf("accepted positional arguments: status=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
			if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("report published on invalid input: %v", err)
			}
			if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("checkpoint directory created on invalid input: %v", err)
			}
		})
	}
}

func TestSocialFoodCLINeverClobbersExistingOutputs(t *testing.T) {
	binary := buildSocialFoodCLI(t)
	run := func(args ...string) (string, error) {
		cmd := exec.Command(binary, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		return stderr.String(), err
	}
	t.Run("existing-checkpoint-dir", func(t *testing.T) {
		base := t.TempDir()
		dir := filepath.Join(base, "bundles")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		sentinel := filepath.Join(dir, "sentinel")
		if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(base, "report.json")
		stderr, err := run("-q=3", "-hours=1", "-out="+out, "-checkpoint-dir="+dir)
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() == 0 {
			t.Fatalf("accepted an existing checkpoint directory: %v stderr=%q", err, stderr)
		}
		if !bytes.Contains([]byte(stderr), []byte("already exists")) {
			t.Fatalf("unclear refusal: %q", stderr)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 || entries[0].Name() != "sentinel" {
			t.Fatalf("existing checkpoint directory modified: %v %v", entries, err)
		}
		if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("report published despite refused directory: %v", err)
		}
	})
	t.Run("existing-out", func(t *testing.T) {
		base := t.TempDir()
		out := filepath.Join(base, "report.json")
		if err := os.WriteFile(out, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(base, "bundles")
		stderr, err := run("-q=3", "-hours=1", "-out="+out, "-checkpoint-dir="+dir)
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() == 0 {
			t.Fatalf("accepted an existing report path: %v stderr=%q", err, stderr)
		}
		content, err := os.ReadFile(out)
		if err != nil || string(content) != "keep" {
			t.Fatalf("existing report modified: %q %v", content, err)
		}
		if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("checkpoint directory created despite refused output: %v", err)
		}
	})
}
