package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesSingleMarkdownOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "skills.yaml"), `
levels: [Beginner]
references:
  - name: A Tour of Go
    url: https://go.dev/tour/
    topics: [Beginner]
categories: {}
skills:
  - slug: read_simple_code
    label: Can read simple code
    level: Beginner
    kind: skill
`)
	writeTestFile(t, filepath.Join(dir, "tests", "read_code", "scorecard.yaml"), `
name: read_code
summary: >
  Read a small function.
assesses:
  - read_simple_code
demonstrated: []
passed: false
attempts: []
`)

	out := filepath.Join(dir, "README.md")
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"--skills", filepath.Join(dir, "skills.yaml"),
		"--tests", filepath.Join(dir, "tests"),
		"--out", out,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() returned error: %v\nstderr:\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote "+out) {
		t.Fatalf("stdout = %q, want write confirmation for %s", stdout.String(), out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("expected Markdown output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills.html")); !os.IsNotExist(err) {
		t.Fatalf("skills.html exists or stat failed unexpectedly: %v", err)
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--out-format", "html"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("run() returned nil, want a flag-parsing error")
	}
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(data)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}
