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

func TestRenderMarkdownEscapesDynamicContent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "README.md")
	data := reportData{
		Levels: []LevelView{{
			Name: "Beginner|Level",
			Skills: []SkillView{{
				Label:         "Can read | explain",
				Kind:          kindSkill,
				Confidence:    30,
				HasData:       true,
				EvidenceTests: []string{"spot|bug"},
				Emoji:         colorEmoji(colorRed),
			}},
			Categories: []CategoryView{{
				Label:      "Core|Skills",
				HasData:    false,
				Emoji:      colorEmoji(colorGrey),
				Skills:     []SkillView{{Label: "Nested|Skill", Kind: kindMilestone, StatusText: "not yet", Emoji: colorEmoji(colorGrey)}},
				Confidence: 0,
			}},
		}},
		References: []ReferenceView{{
			Title:       "Ref | Name",
			URL:         "https://example.test/a file(1)",
			AuthorsText: "A | B",
		}},
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
		"| 🔴 | Can read \\| explain | 30 (spot\\|bug) |",
		"| ⚪ | **Core\\|Skills** | untested |",
		"[Ref \\| Name](https://example.test/a%20file%281%29), A \\| B",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, got)
		}
	}
}
