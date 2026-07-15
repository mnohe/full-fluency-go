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
			Name:      "spot_the_bug",
			DirName:   "spot_the_bug",
			TestID:    "spot_the_bug",
			MetaPath:  filepath.Join("tests", "spot_the_bug", "metadata.yaml"),
			ScorePath: filepath.Join("tests", "spot_the_bug", "scorecard.yaml"),
			Summary:   "Find the bug.",
			Assesses:  []string{"read_basic_code"},
			Attempts: []Attempt{{
				Date:  "2026-07-14T09:00",
				Tiers: []SkillTier{{Skill: "read_basic_code", Tier: stateYellow}},
				Notes: "Passed with edge cases.",
			}},
			Passed: true,
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
			Name:         "Bad Name",
			DirName:      "score one",
			TestID:       "also_wrong",
			MetaPath:     filepath.Join("tests", "score one", "metadata.yaml"),
			ScorePath:    filepath.Join("tests", "score one", "scorecard.yaml"),
			Summary:      "",
			Assesses:     []string{"foundations"},
			Demonstrated: []string{"nonexistent_skill"},
			Attempts: []Attempt{{
				Date:  "14-07-2026",
				Tiers: []SkillTier{{Skill: "read_basic_code", Tier: tierFail}},
				Notes: "",
			}},
			Passed: true,
		},
	}

	err := validateInputs(sf, cards)
	if err == nil {
		t.Fatal("validateInputs() returned nil, want validation error")
	}
	for _, want := range []string{
		`directory name must match`,
		`name "Bad Name" must match directory "score one"`,
		"summary is required",
		`references category "foundations"`,
		`references unknown skill slug "nonexistent_skill"`,
		`test_id "also_wrong" must match directory "score one"`,
		`date "14-07-2026" must use YYYY-MM-DDTHH:MM`,
		`skill "read_basic_code" is not in this test's assesses/demonstrated`,
		"notes is required unless every graded skill is a clean green-tier pass",
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
  - slug: read_basic_code
    label: Can read simple code
    level: Beginner
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
			{
				Slug:  "read_basic_code",
				Label: "Can read simple code",
				Rubric: Rubric{
					Orange: "Explains a short program correctly.",
					Yellow: "Explains a short program with conditionals correctly.",
					Green:  "Explains a short program with structs correctly.",
				},
				Level:      "Beginner",
				Categories: []string{"foundations"},
			},
			{
				Slug:   "done_thing",
				Label:  "Can do the thing",
				Rubric: Rubric{Green: "Does the thing."},
				Level:  "Beginner",
			},
		},
	}
}

func TestValidateRubricRequiresGreen(t *testing.T) {
	sf := minimalSkillsFile()
	sf.Skills = append(sf.Skills, Skill{Slug: "no_rubric", Label: "Missing rubric", Level: "Beginner"})

	err := validateInputs(sf, nil)
	if err == nil {
		t.Fatal("validateInputs() returned nil, want a missing-rubric error")
	}
	if want := "rubric.green is required"; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func TestValidateRubricRejectsYellowWithoutOrange(t *testing.T) {
	sf := minimalSkillsFile()
	sf.Skills = append(sf.Skills, Skill{
		Slug:   "skipped_orange",
		Label:  "Skips orange",
		Rubric: Rubric{Yellow: "Some yellow bar.", Green: "Some green bar."},
		Level:  "Beginner",
	})

	err := validateInputs(sf, nil)
	if err == nil {
		t.Fatal("validateInputs() returned nil, want a skipped-tier error")
	}
	if want := "rubric.yellow is set without rubric.orange"; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func baseTestCard() Scorecard {
	return Scorecard{
		Name:      "spot_the_bug",
		DirName:   "spot_the_bug",
		TestID:    "spot_the_bug",
		MetaPath:  filepath.Join("tests", "spot_the_bug", "metadata.yaml"),
		ScorePath: filepath.Join("tests", "spot_the_bug", "scorecard.yaml"),
		Summary:   "Find the bug.",
		Assesses:  []string{"read_basic_code"},
	}
}

func TestValidateAttemptRejectsInvalidTier(t *testing.T) {
	sf := minimalSkillsFile()
	card := baseTestCard()
	card.Attempts = []Attempt{{
		Date:  "2026-07-14T09:00",
		Tiers: []SkillTier{{Skill: "read_basic_code", Tier: "maybe"}},
		Notes: "n/a",
	}}
	card.Passed = true

	err := validateInputs(sf, []Scorecard{card})
	if err == nil {
		t.Fatal("validateInputs() returned nil, want an invalid-tier error")
	}
	if want := `tier "maybe" must be`; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func TestValidateAttemptRejectsTierNotDefinedByRubric(t *testing.T) {
	sf := minimalSkillsFile()
	card := baseTestCard()
	card.Assesses = []string{"done_thing"} // done_thing's rubric only defines green
	card.Attempts = []Attempt{{
		Date:  "2026-07-14T09:00",
		Tiers: []SkillTier{{Skill: "done_thing", Tier: stateOrange}},
		Notes: "n/a",
	}}
	card.Passed = true

	err := validateInputs(sf, []Scorecard{card})
	if err == nil {
		t.Fatal("validateInputs() returned nil, want a tier-not-defined error")
	}
	if want := `tier "orange" is not defined in "done_thing"'s rubric`; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func TestValidateAttemptRejectsSkillNotAssessedOrDemonstrated(t *testing.T) {
	sf := minimalSkillsFile()
	card := baseTestCard()
	card.Attempts = []Attempt{{
		Date:  "2026-07-14T09:00",
		Tiers: []SkillTier{{Skill: "done_thing", Tier: stateGreen}}, // not in assesses/demonstrated
		Notes: "n/a",
	}}
	card.Passed = true

	err := validateInputs(sf, []Scorecard{card})
	if err == nil {
		t.Fatal("validateInputs() returned nil, want a not-relevant error")
	}
	if want := `skill "done_thing" is not in this test's assesses/demonstrated`; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func TestValidateAttemptRejectsDuplicateSkillInOneAttempt(t *testing.T) {
	sf := minimalSkillsFile()
	card := baseTestCard()
	card.Attempts = []Attempt{{
		Date: "2026-07-14T09:00",
		Tiers: []SkillTier{
			{Skill: "read_basic_code", Tier: stateGreen},
			{Skill: "read_basic_code", Tier: stateOrange},
		},
		Notes: "n/a",
	}}
	card.Passed = true

	err := validateInputs(sf, []Scorecard{card})
	if err == nil {
		t.Fatal("validateInputs() returned nil, want a duplicate-skill error")
	}
	if want := `duplicate skill "read_basic_code"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func TestValidateAttemptRequiresAtLeastOneTier(t *testing.T) {
	sf := minimalSkillsFile()
	card := baseTestCard()
	card.Attempts = []Attempt{{Date: "2026-07-14T09:00", Notes: "n/a"}}
	card.Passed = false

	err := validateInputs(sf, []Scorecard{card})
	if err == nil {
		t.Fatal("validateInputs() returned nil, want a no-tiers error")
	}
	if want := "tiers must grade at least one skill"; !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error missing %q:\n%v", want, err)
	}
}

func TestValidateAttemptNotesRequiredUnlessAllGreen(t *testing.T) {
	sf := minimalSkillsFile()
	sf.Skills = append(sf.Skills, Skill{
		Slug:   "second_skill",
		Label:  "Second skill",
		Rubric: Rubric{Green: "Does the second thing."},
		Level:  "Beginner",
	})

	allGreen := baseTestCard()
	allGreen.Assesses = []string{"read_basic_code", "second_skill"}
	allGreen.Attempts = []Attempt{{
		Date: "2026-07-14T09:00",
		Tiers: []SkillTier{
			{Skill: "read_basic_code", Tier: stateGreen},
			{Skill: "second_skill", Tier: stateGreen},
		},
		Notes: "",
	}}
	allGreen.Passed = true
	if err := validateInputs(sf, []Scorecard{allGreen}); err != nil {
		t.Fatalf("validateInputs() returned unexpected error when every graded skill is green:\n%v", err)
	}

	oneNotGreen := baseTestCard()
	oneNotGreen.Assesses = []string{"read_basic_code", "second_skill"}
	oneNotGreen.Attempts = []Attempt{{
		Date: "2026-07-14T09:00",
		Tiers: []SkillTier{
			{Skill: "read_basic_code", Tier: stateGreen},
			{Skill: "second_skill", Tier: stateOrange},
		},
		Notes: "",
	}}
	oneNotGreen.Passed = true
	err := validateInputs(sf, []Scorecard{oneNotGreen})
	if err == nil || !strings.Contains(err.Error(), "notes is required") {
		t.Fatalf("validateInputs() = %v, want a notes-required error when one graded skill isn't green", err)
	}

	fail := baseTestCard()
	fail.Attempts = []Attempt{{
		Date:  "2026-07-14T09:00",
		Tiers: []SkillTier{{Skill: "read_basic_code", Tier: tierFail}},
		Notes: "",
	}}
	fail.Passed = false
	if err := validateInputs(sf, []Scorecard{fail}); err == nil || !strings.Contains(err.Error(), "notes is required") {
		t.Fatalf("validateInputs() = %v, want a notes-required error for a fail with no notes", err)
	}
}
