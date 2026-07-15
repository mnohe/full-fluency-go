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
	Badge      BadgeView
	Levels     []LevelView
	References []ReferenceView
	TestIndex  []TestIndexEntry
}

// BadgeView is the report's single top-of-page achievement badge: the
// highest skills.yaml level fully cleared (every one of its skills at
// green or better) cascading from the first level, so a later level only
// counts once every earlier one does too. Master isn't one of skills.yaml's
// own levels; it's a separate rank above the last of them, earned only once
// every skill in every level has reached the mastered rung (see
// stateMaster in confidence.go).
type BadgeView struct {
	Level string
	Alt   string
	URL   string
}

// levelBadgeColors assigns each skills.yaml level a fixed shields.io color,
// by position in the levels list. An achieved level beyond this palette's
// length (skills.yaml grew a fifth non-Master level) falls back to the last
// color rather than erroring, since the badge is cosmetic.
var levelBadgeColors = []string{colorOrange, colorYellow, colorGreen, "blue"}

// masterBadgeColor is black, echoing a martial-arts black belt: Master is
// the rank above every colored level, not just another rung in their
// sequence.
const masterBadgeColor = "black"

// computeBadge walks the already-rendered level views -- no need to
// recompute skill state, buildLevelViews already resolved every skill's
// Color -- to find the badge this report should show.
func computeBadge(levels []LevelView) BadgeView {
	achievedIdx := -1
	totalSkills := 0
	allMastered := true

	for i, level := range levels {
		complete := len(level.Skills) > 0
		for _, skill := range level.Skills {
			totalSkills++
			if skill.Color != colorGreen && skill.Color != colorGold {
				complete = false
			}
			if skill.Color != colorGold {
				allMastered = false
			}
		}
		if complete && achievedIdx == i-1 {
			achievedIdx = i
		}
	}
	if totalSkills == 0 {
		allMastered = false
	}

	label := "FF:GO"
	if allMastered {
		return newBadgeView(label, "Master", masterBadgeColor)
	}
	if achievedIdx == -1 {
		name := "Beginner"
		if len(levels) > 0 {
			name = levels[0].Name
		}
		return newBadgeView(label, name, colorGrey)
	}

	color := levelBadgeColors[len(levelBadgeColors)-1]
	if achievedIdx < len(levelBadgeColors) {
		color = levelBadgeColors[achievedIdx]
	}
	return newBadgeView(label, levels[achievedIdx].Name, color)
}

// newBadgeView builds a badge's Markdown alt text and shields.io URL
// together so the two can never drift apart.
func newBadgeView(label, level, color string) BadgeView {
	return BadgeView{
		Level: level,
		Alt:   fmt.Sprintf("%s level: %s", label, level),
		URL:   shieldsBadgeURL(label, level, color),
	}
}

// shieldsBadgeURL builds a static shields.io badge URL from a label,
// message, and color, e.g.
// https://img.shields.io/badge/FF%3AGO-Beginner-orange?style=for-the-badge.
func shieldsBadgeURL(label, message, color string) string {
	return fmt.Sprintf("https://img.shields.io/badge/%s-%s-%s?style=for-the-badge",
		shieldsEscape(label), shieldsEscape(message), shieldsEscape(color))
}

// shieldsEscape encodes one shields.io badge path segment, following
// shields' own convention (-- for a literal -, __ for a literal _, _ for a
// space) and percent-encoding anything else that isn't safe unescaped in a
// URL path segment.
func shieldsEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '-':
			b.WriteString("--")
		case r == '_':
			b.WriteString("__")
		case r == ' ':
			b.WriteByte('_')
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			for _, c := range []byte(string(r)) {
				fmt.Fprintf(&b, "%%%02X", c)
			}
		}
	}
	return b.String()
}

// colorEmoji maps status colors onto GitHub-rendered emoji, the only color
// mechanism available in the Markdown report without external assets.
func colorEmoji(color string) string {
	switch color {
	case colorRed:
		return "\U0001F534" // 🔴
	case colorOrange:
		return "\U0001F7E0" // 🟠
	case colorYellow:
		return "\U0001F7E1" // 🟡
	case colorGreen:
		return "\U0001F7E2" // 🟢
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
		"imageAlt": markdownImageAlt,
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

// markdownImageAlt escapes text used inside Markdown image alt brackets.
func markdownImageAlt(s string) string {
	return markdownLinkText(s)
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
