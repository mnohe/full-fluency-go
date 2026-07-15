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
  - slug: read_basic_code
    label: Can read simple code
    rubric:
      green: >
        Reads a short function aloud and correctly explains what it does.
    level: Beginner
`)
	writeTestFile(t, filepath.Join(dir, "tests", "read_code", "metadata.yaml"), `
name: read_code
summary: >
  Read a small function.
assesses:
  - read_basic_code
`)
	writeTestFile(t, filepath.Join(dir, "tests", "read_code", "scorecard.yaml"), `
test_id: read_code
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
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if want := "![FF:GO level: Beginner](https://img.shields.io/badge/FF%3AGO-Beginner-grey?style=for-the-badge)"; !strings.Contains(string(b), want) {
		t.Fatalf("generated report missing badge %q:\n%s", want, string(b))
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
