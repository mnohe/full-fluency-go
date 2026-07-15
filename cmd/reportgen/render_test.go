package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownEscaping(t *testing.T) {
	if got, want := markdownTableCell(`a|b\c`+"\nnext"), `a\|b\\c next`; got != want {
		t.Fatalf("markdownTableCell() = %q, want %q", got, want)
	}
	if got, want := markdownLink("evi[dence]", "https://example.test/a file(1)"), `[evi\[dence\]](https://example.test/a%20file%281%29)`; got != want {
		t.Fatalf("markdownLink() = %q, want %q", got, want)
	}
}

func TestBuildTestIndexNumbersOnlyAttemptedTests(t *testing.T) {
	cards := []Scorecard{
		{Name: "alpha", Attempts: []Attempt{{Tiers: []SkillTier{{Skill: "s", Tier: stateGreen}}}}},
		{Name: "no_attempts_yet"},
		{Name: "beta", Attempts: []Attempt{{Tiers: []SkillTier{{Skill: "s", Tier: stateYellow}}}}},
	}

	numbers, index := buildTestIndex(cards)
	if got, want := numbers["alpha"], 1; got != want {
		t.Fatalf("numbers[alpha] = %d, want %d", got, want)
	}
	if got, want := numbers["beta"], 2; got != want {
		t.Fatalf("numbers[beta] = %d, want %d", got, want)
	}
	if _, exists := numbers["no_attempts_yet"]; exists {
		t.Fatal("buildTestIndex() numbered a test with no attempts")
	}
	if len(index) != 2 {
		t.Fatalf("len(index) = %d, want 2", len(index))
	}
}

func TestRenderMarkdownEscapesDynamicContent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "README.md")
	data := reportData{
		Levels: []LevelView{{
			Name: "Beginner|Level",
			Skills: []SkillView{{
				Label:          "Can read | explain",
				StatusText:     "red",
				HasData:        true,
				CategoryLabels: []string{"Core|Skills"},
				Evidence:       []EvidenceLink{{Number: 1, TestID: "spot_bug"}},
				Emoji:          colorEmoji(colorRed),
			}},
		}},
		References: []ReferenceView{{
			Title:       "Ref | Name",
			URL:         "https://example.test/a file(1)",
			AuthorsText: "A | B",
		}},
		TestIndex: []TestIndexEntry{{Number: 1, TestID: "spot_bug"}},
	}

	if err := renderMarkdown(out, data); err != nil {
		t.Fatalf("renderMarkdown() returned error: %v", err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		"## Beginner\\|Level",
		"| 🔴 | Can read \\| explain | Core\\|Skills | red | [1](tests/spot_bug/) |",
		"1. [spot_bug](tests/spot_bug/)",
		"[Ref \\| Name](https://example.test/a%20file%281%29), A \\| B",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, got)
		}
	}
}
