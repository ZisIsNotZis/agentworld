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

func buildReproductionCLI(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "reproduction")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, out)
	}
	return binary
}

func TestReproductionCLIRejectsPositionalArgumentsWithoutPublication(t *testing.T) {
	binary := buildReproductionCLI(t)
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

func TestReproductionCLINoClobberPublication(t *testing.T) {
	binary := buildReproductionCLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	base := t.TempDir()
	dir := filepath.Join(base, "bundles")
	out := filepath.Join(base, "report.json")
	run := func(args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, binary, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("run %v: %v: %s", args, err, stderr.String())
		}
		return cmd
	}
	run("-q=0", "-hours=1", "-births=disabled", "-out="+out, "-checkpoint-dir="+dir)
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("report not published: %v", err)
	}
	// A second run over either artifact is refused with no publication.
	if err := exec.CommandContext(ctx, binary, "-q=0", "-hours=1", "-births=disabled", "-out="+out, "-checkpoint-dir="+filepath.Join(base, "other")).Run(); err == nil {
		t.Fatal("existing output accepted")
	}
	if err := exec.CommandContext(ctx, binary, "-q=0", "-hours=1", "-births=disabled", "-out="+filepath.Join(base, "second.json"), "-checkpoint-dir="+dir).Run(); err == nil {
		t.Fatal("existing checkpoint directory accepted")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The first run published exactly the neutral root and one branch child;
	// the refused runs added nothing.
	if len(entries) != 2 {
		t.Fatalf("checkpoint directory churn: %d entries", len(entries))
	}
	// The -genome-variant refusal must also publish nothing.
	variantDir := filepath.Join(base, "variant-bundles")
	if err := exec.CommandContext(ctx, binary, "-q=8", "-hours=1", "-births=disabled", "-genome-variant", "-out="+filepath.Join(base, "variant.json"), "-checkpoint-dir="+variantDir).Run(); err == nil {
		t.Fatal("variant disabled cell accepted")
	}
	if _, err := os.Stat(variantDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("variant refusal created artifacts: %v", err)
	}
}
