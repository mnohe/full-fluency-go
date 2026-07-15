package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

//go:embed report.md.tmpl
var templateFS embed.FS

const (
	colorGrey   = "grey"
	colorRed    = "red"
	colorOrange = "orange"
	colorYellow = "yellow"
	colorGreen  = "green"
	colorGold   = "gold"
)

// EvidenceLink is one contributing test, numbered against the report's
// global test index so the Evidence column can show a compact "[1] [2]"
// instead of repeating test IDs inline in every row.
type EvidenceLink struct {
	Number int
	TestID string
}

// SkillView is the template-facing form of a registry skill: every skill is
// independently scoreable, so this carries no notion of category rollup or
// milestone state -- categories are a plain display column, not a grouping.
type SkillView struct {
	Slug           string
	Label          string
	StatusText     string
	HasData        bool
	CategoryLabels []string
	Evidence       []EvidenceLink
	Color          string
	Emoji          string
}

// CategoriesText joins CategoryLabels for the report's Category column.
func (v SkillView) CategoriesText() string {
	return strings.Join(v.CategoryLabels, ", ")
}

// LevelView is one top-level report section in skills.yaml level order.
type LevelView struct {
	Name   string
	Skills []SkillView
}

// ReferenceView is one bibliography row, already normalized for display.
type ReferenceView struct {
	Title       string
	AuthorsText string
	URL         string
}

// TestIndexEntry is one row of the report's Tests appendix -- the number the
// Evidence column links to, resolved to the actual test.
type TestIndexEntry struct {
	Number int
	TestID string
}

// reportData is the complete template input for the single generated output.
type reportData struct {
	Levels     []LevelView
	References []ReferenceView
	TestIndex  []TestIndexEntry
}

// colorEmoji maps status colors onto GitHub-rendered emoji, the only color
// mechanism available in the Markdown report without external assets.
func colorEmoji(color string) string {
	switch color {
	case colorRed:
		return "\U0001F534"
	case colorOrange:
		return "\U0001F7E0"
	case colorYellow:
		return "\U0001F7E1"
	case colorGreen:
		return "\U0001F7E2"
	case colorGold:
		return "⭐"
	default:
		return "⚪"
	}
}

// stateStatusText is the report's display word for a skill's ladder state.
func stateStatusText(hasData bool, state string) string {
	if !hasData {
		return "untested"
	}
	if state == stateMaster {
		return "mastered"
	}
	return state // "red"/"orange"/"yellow"/"green" already read fine as words
}

// buildTestIndex numbers every test that has contributed at least one
// attempt, in the order loadScorecards already returns them (lexical by
// directory name), so the Evidence column and the Tests appendix agree.
func buildTestIndex(cards []Scorecard) (numbers map[string]int, index []TestIndexEntry) {
	numbers = map[string]int{}
	for _, card := range cards {
		if len(card.Attempts) == 0 {
			continue
		}
		n := len(numbers) + 1
		numbers[card.Name] = n
		index = append(index, TestIndexEntry{Number: n, TestID: card.Name})
	}
	return numbers, index
}

// buildSkillView computes one skill's display state, including its
// evidence trail resolved against the report's global test numbering.
func buildSkillView(skill Skill, cards []Scorecard, categories map[string]Category, testNumbers map[string]int) SkillView {
	state, hasData, evidenceTests := skillLadder(skill.Slug, cards)

	view := SkillView{
		Slug:    skill.Slug,
		Label:   skill.Label,
		HasData: hasData,
	}
	for _, categorySlug := range skill.Categories {
		view.CategoryLabels = append(view.CategoryLabels, categories[categorySlug].Label)
	}
	for _, testID := range evidenceTests {
		n, numbered := testNumbers[testID]
		if !numbered {
			continue // defensive: skillLadder and buildTestIndex agree on what counts as evidence, but never silently render a fabricated [0] link if that ever drifts
		}
		view.Evidence = append(view.Evidence, EvidenceLink{Number: n, TestID: testID})
	}

	if hasData {
		view.Color = state
	} else {
		view.Color = colorGrey
	}
	view.StatusText = stateStatusText(hasData, state)
	view.Emoji = colorEmoji(view.Color)
	return view
}

// buildLevelViews preserves the human-authored ordering from skills.yaml.
// There is no category grouping here: every skill is its own row regardless
// of which categories (if any) it carries.
func buildLevelViews(sf SkillsFile, cards []Scorecard, testNumbers map[string]int) []LevelView {
	levels := make([]LevelView, 0, len(sf.Levels))
	for _, levelName := range sf.Levels {
		level := LevelView{Name: levelName}
		for _, skill := range sf.Skills {
			if skill.Level != levelName {
				continue
			}
			level.Skills = append(level.Skills, buildSkillView(skill, cards, sf.Categories, testNumbers))
		}
		levels = append(levels, level)
	}
	return levels
}

// buildReferenceViews joins author lists once so the template does not need to
// know about bibliography structure.
func buildReferenceViews(refs []Reference) []ReferenceView {
	views := make([]ReferenceView, 0, len(refs))
	for _, ref := range refs {
		view := ReferenceView{Title: ref.Name, URL: ref.URL}
		if len(ref.Authors) > 0 {
			view.AuthorsText = strings.Join(ref.Authors, " & ")
		}
		views = append(views, view)
	}
	return views
}

// renderMarkdown renders into memory before touching the destination file. A
// template error therefore cannot leave README.md half-written.
func renderMarkdown(outPath string, data reportData) error {
	tmpl, err := template.New("report.md.tmpl").Funcs(template.FuncMap{
		"cell":     markdownTableCell,
		"link":     markdownLink,
		"evidence": markdownEvidenceCell,
	}).ParseFS(templateFS, "report.md.tmpl")
	if err != nil {
		return fmt.Errorf("parse Markdown template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute Markdown template: %w", err)
	}
	if err := writeFileSafely(outPath, buf.Bytes()); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// writeFileSafely writes a fully rendered file through a sibling temporary
// file. On POSIX, os.Rename replaces the destination atomically; on Windows,
// the standard library cannot atomically replace an existing file, so the
// fallback removes only after the temp file has been fully written and closed.
func writeFileSafely(path string, data []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	mode := fileModeForReplacement(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := replaceFile(tmpPath, path); err != nil {
		return err
	}
	keepTemp = true
	return nil
}

// fileModeForReplacement preserves an existing report's permissions and uses a
// conventional read-friendly mode when creating the report for the first time.
func fileModeForReplacement(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return 0o644
	}
	return info.Mode().Perm()
}

// replaceFile performs the final move from temp path to destination. POSIX gets
// atomic replacement from os.Rename; Windows gets the safest stdlib fallback we
// can offer without platform-specific syscalls.
func replaceFile(tmpPath, finalPath string) error {
	if err := os.Rename(tmpPath, finalPath); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}
	if err := os.Remove(finalPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, finalPath)
}

// markdownEvidenceCell renders a skill's evidence trail as compact linked
// numbers, e.g. "[1](tests/spot_the_bug/) [2](tests/read_and_explain/)" --
// concise in the row, fully traceable via the Tests appendix and the link
// destination itself.
func markdownEvidenceCell(skill SkillView) string {
	if len(skill.Evidence) == 0 {
		return ""
	}
	parts := make([]string, len(skill.Evidence))
	for i, e := range skill.Evidence {
		parts[i] = fmt.Sprintf("[%d](tests/%s/)", e.Number, e.TestID)
	}
	return strings.Join(parts, " ")
}

// markdownLink renders a Markdown link after escaping the link text and the
// small set of URL characters that commonly break inline link destinations.
func markdownLink(text, rawURL string) string {
	if rawURL == "" {
		return markdownTableCell(text)
	}
	return "[" + markdownLinkText(text) + "](" + markdownLinkDestination(rawURL) + ")"
}

// markdownTableCell escapes dynamic text for GitHub-flavored Markdown tables.
func markdownTableCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.TrimSpace(s)
}

// markdownLinkText adds link-text escaping on top of table-cell escaping.
func markdownLinkText(s string) string {
	s = markdownTableCell(s)
	s = strings.ReplaceAll(s, "[", `\[`)
	s = strings.ReplaceAll(s, "]", `\]`)
	return s
}

// markdownLinkDestination keeps inline Markdown links intact for validated
// absolute URLs that contain spaces or parentheses.
func markdownLinkDestination(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "(", "%28")
	s = strings.ReplaceAll(s, ")", "%29")
	return s
}
