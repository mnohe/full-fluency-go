package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateInputsAcceptsMinimalValidRegistry(t *testing.T) {
	sf := minimalSkillsFile()
	cards := []Scorecard{
		{
			Name:     "spot_the_bug",
			TestID:   "spot_the_bug",
			Path:     filepath.Join("tests", "spot_the_bug", "scorecard.yaml"),
			Summary:  "Find the bug.",
			Assesses: []string{"read_simple_code"},
			Attempts: []Attempt{{Date: "2026-07-14", Score: 85, Confidence: confidenceMedium, Notes: "Passed with edge cases."}},
			Passed:   true,
		},
	}

	if err := validateInputs(sf, cards); err != nil {
		t.Fatalf("validateInputs() returned unexpected error:\n%v", err)
	}
}

func TestValidateInputsReportsProtocolViolations(t *testing.T) {
	sf := minimalSkillsFile()
	cards := []Scorecard{
		{
			Name:         "bad_name",
			TestID:       "score_one",
			Path:         filepath.Join("tests", "score_one", "scorecard.yaml"),
			Summary:      "",
			Assesses:     []string{"foundations"},
			Demonstrated: []string{"done_thing"},
			Attempts:     []Attempt{{Date: "14-07-2026", Score: 101, Confidence: "certain", Notes: ""}},
			Passed:       true,
		},
	}

	err := validateInputs(sf, cards)
	if err == nil {
		t.Fatal("validateInputs() returned nil, want validation error")
	}
	for _, want := range []string{
		`name "bad_name" must match directory "score_one"`,
		"summary is required",
		`references category "foundations"`,
		`references milestone "done_thing"`,
		`date "14-07-2026" must use YYYY-MM-DD`,
		"score 101 must be between 0 and 100",
		`confidence "certain" must be`,
		"passed is true, want false",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("validation error missing %q:\n%v", want, err)
		}
	}
}

func TestLoadSkillsRejectsUnknownYAMLFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skills.yaml")
	data := []byte(`
levels: [Beginner]
categories: {}
skills:
  - slug: read_simple_code
    label: Can read simple code
    level: Beginner
    kind: skill
    typo_field: nope
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := loadSkills(path)
	if err == nil {
		t.Fatal("loadSkills() returned nil, want unknown-field error")
	}
	if !strings.Contains(err.Error(), "typo_field") {
		t.Fatalf("loadSkills() error = %v, want mention of typo_field", err)
	}
}

func minimalSkillsFile() SkillsFile {
	return SkillsFile{
		Levels: []string{"Beginner"},
		References: []Reference{{
			Name:   "A Tour of Go",
			URL:    "https://go.dev/tour/",
			Topics: []string{"Beginner"},
		}},
		Categories: map[string]Category{
			"foundations": {Label: "Foundations"},
		},
		Skills: []Skill{
			{Slug: "read_simple_code", Label: "Can read simple code", Level: "Beginner", Kind: kindSkill, Categories: []string{"foundations"}},
			{Slug: "done_thing", Label: "Has done thing", Level: "Beginner", Kind: kindMilestone, Achieved: false},
		},
	}
}
