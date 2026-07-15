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

func skillView(color string) SkillView {
	return SkillView{HasData: color != colorGrey, Color: color}
}

func TestComputeBadgeNoLevelClearedYetShowsFirstLevelGrey(t *testing.T) {
	levels := []LevelView{
		{Name: "Beginner", Skills: []SkillView{skillView(colorYellow), skillView(colorGreen)}},
		{Name: "Intermediate", Skills: []SkillView{skillView(colorGreen)}},
	}
	got := computeBadge(levels)
	if got.Level != "Beginner" {
		t.Fatalf("Level = %q, want %q", got.Level, "Beginner")
	}
	if !strings.Contains(got.URL, "-Beginner-grey?") {
		t.Fatalf("URL = %q, want grey Beginner badge", got.URL)
	}
}

func TestComputeBadgeFirstLevelClearedIsOrange(t *testing.T) {
	levels := []LevelView{
		{Name: "Beginner", Skills: []SkillView{skillView(colorGreen), skillView(colorGold)}},
		{Name: "Intermediate", Skills: []SkillView{skillView(colorYellow)}},
	}
	got := computeBadge(levels)
	if got.Level != "Beginner" || !strings.Contains(got.URL, "-Beginner-orange?") {
		t.Fatalf("computeBadge() = %+v, want cleared Beginner/orange", got)
	}
}

func TestComputeBadgeDoesNotSkipAnUnclearedEarlierLevel(t *testing.T) {
	levels := []LevelView{
		{Name: "Beginner", Skills: []SkillView{skillView(colorRed)}},
		{Name: "Intermediate", Skills: []SkillView{skillView(colorGreen)}},
	}
	got := computeBadge(levels)
	if got.Level != "Beginner" || !strings.Contains(got.URL, "grey") {
		t.Fatalf("computeBadge() = %+v, want Beginner/grey even though Intermediate alone is all green", got)
	}
}

func TestComputeBadgeAllMasteredIsBlackMaster(t *testing.T) {
	levels := []LevelView{
		{Name: "Beginner", Skills: []SkillView{skillView(colorGold)}},
		{Name: "Expert", Skills: []SkillView{skillView(colorGold)}},
	}
	got := computeBadge(levels)
	if got.Level != "Master" || !strings.Contains(got.URL, "-Master-black?") {
		t.Fatalf("computeBadge() = %+v, want Master/black", got)
	}
}

func TestComputeBadgeNoSkillsAtAllIsGrey(t *testing.T) {
	got := computeBadge(nil)
	if got.Level != "Beginner" || !strings.Contains(got.URL, "grey") {
		t.Fatalf("computeBadge(nil) = %+v, want Beginner/grey", got)
	}
}

func TestShieldsBadgeURLEscapesReservedCharacters(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		message string
		color   string
		want    string
	}{
		{
			name:    "ascii reserved punctuation",
			label:   "FF:GO",
			message: "Beginner",
			color:   "orange",
			want:    "https://img.shields.io/badge/FF%3AGO-Beginner-orange?style=for-the-badge",
		},
		{
			name:    "shields conventions and utf8",
			label:   "FF_GO",
			message: "Début - A_B",
			color:   "blue",
			want:    "https://img.shields.io/badge/FF__GO-D%C3%A9but_--_A__B-blue?style=for-the-badge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shieldsBadgeURL(tt.label, tt.message, tt.color); got != tt.want {
				t.Fatalf("shieldsBadgeURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderMarkdownEscapesDynamicContent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "README.md")
	data := reportData{
		Badge: BadgeView{Alt: "FF:[GO] level: Beginner", URL: "https://img.shields.io/badge/FF%3AGO-Beginner-grey?style=for-the-badge"},
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
		"![FF:\\[GO\\] level: Beginner](https://img.shields.io/badge/FF%3AGO-Beginner-grey?style=for-the-badge)",
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
