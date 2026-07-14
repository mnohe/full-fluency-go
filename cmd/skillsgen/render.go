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

// SkillView is the template-facing form of a registry skill. It carries both
// source identity and computed display state so the template can stay mostly
// declarative.
type SkillView struct {
	Slug          string
	Label         string
	Kind          string
	Confidence    int
	HasData       bool
	Achieved      bool
	Evidence      string
	EvidenceTests []string
	StatusText    string
	Color         string
	Emoji         string
}

// EvidenceTestsText joins contributing test IDs for compact Markdown display.
func (v SkillView) EvidenceTestsText() string {
	return strings.Join(v.EvidenceTests, ", ")
}

// CategoryView is a category plus the minimum confidence across its member
// skills. Untested members count as zero for the category gate.
type CategoryView struct {
	Slug       string
	Label      string
	Confidence int
	HasData    bool
	Color      string
	Emoji      string
	Skills     []SkillView
}

// LevelView is one top-level report section in skills.yaml level order.
type LevelView struct {
	Name       string
	Skills     []SkillView
	Categories []CategoryView
}

// ReferenceView is one bibliography row, already normalized for display.
type ReferenceView struct {
	Title       string
	AuthorsText string
	URL         string
}

// reportData is the complete template input for the single generated output.
type reportData struct {
	Levels     []LevelView
	References []ReferenceView
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

// buildSkillView computes the display state for one skill. Milestones skip the
// confidence pipeline because their state is hand-edited, not score-derived.
func buildSkillView(skill Skill, cards []Scorecard) SkillView {
	view := SkillView{
		Slug:     skill.Slug,
		Label:    skill.Label,
		Kind:     skill.Kind,
		Achieved: skill.Achieved,
		Evidence: skill.Evidence,
	}
	if skill.Kind == kindSkill {
		view.Confidence, view.HasData, view.EvidenceTests = skillConfidence(skill.Slug, cards)
	}
	view.StatusText, view.Color = statusBucket(view.Kind, view.Achieved, view.HasData, view.Confidence)
	view.Emoji = colorEmoji(view.Color)
	return view
}

// buildLevelViews preserves the human-authored ordering from skills.yaml while
// materializing category rollups for the report.
func buildLevelViews(sf SkillsFile, cards []Scorecard) []LevelView {
	levels := make([]LevelView, 0, len(sf.Levels))
	for _, levelName := range sf.Levels {
		level := LevelView{Name: levelName}
		categoryOrder := []string{}
		categoryViews := map[string]*CategoryView{}

		for _, skill := range sf.Skills {
			if skill.Level != levelName {
				continue
			}
			skillView := buildSkillView(skill, cards)
			if len(skill.Categories) == 0 {
				level.Skills = append(level.Skills, skillView)
				continue
			}
			for _, categorySlug := range skill.Categories {
				category, ok := categoryViews[categorySlug]
				if !ok {
					meta := sf.Categories[categorySlug]
					category = &CategoryView{Slug: categorySlug, Label: meta.Label}
					categoryViews[categorySlug] = category
					categoryOrder = append(categoryOrder, categorySlug)
				}
				category.Skills = append(category.Skills, skillView)
			}
		}

		for _, categorySlug := range categoryOrder {
			category := categoryViews[categorySlug]
			category.Confidence, category.HasData = categoryConfidence(category.Skills)
			_, category.Color = statusBucket(kindSkill, false, category.HasData, category.Confidence)
			category.Emoji = colorEmoji(category.Color)
			level.Categories = append(level.Categories, *category)
		}
		levels = append(levels, level)
	}
	return levels
}

// categoryConfidence applies the protocol's "no partial credit" category rule:
// the category confidence is the minimum effective confidence of its members.
func categoryConfidence(skills []SkillView) (confidence int, hasData bool) {
	haveCandidate := false
	for _, skill := range skills {
		effective := 0
		if skill.HasData {
			hasData = true
			effective = skill.Confidence
		}
		if !haveCandidate || effective < confidence {
			confidence = effective
			haveCandidate = true
		}
	}
	return confidence, hasData
}

// statusBucket maps a skill/category/milestone state onto the report's fixed
// status language and color band.
func statusBucket(kind string, achieved, hasData bool, confidence int) (statusText, color string) {
	if kind == kindMilestone {
		if achieved {
			return "achieved", colorGreen
		}
		return "not yet", colorGrey
	}
	if !hasData {
		return "untested", colorGrey
	}
	switch {
	case confidence >= 100:
		return "mastered", colorGold
	case confidence >= 90:
		return "green", colorGreen
	case confidence >= 66:
		return "yellow", colorYellow
	case confidence >= 33:
		return "orange", colorOrange
	default:
		return "red", colorRed
	}
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
		"cell":           markdownTableCell,
		"categoryStatus": markdownCategoryStatus,
		"description":    markdownSkillDescription,
		"link":           markdownLink,
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

// markdownCategoryStatus is the report wording for category rollups.
func markdownCategoryStatus(category CategoryView) string {
	if !category.HasData {
		return "untested"
	}
	return fmt.Sprintf("%d", category.Confidence)
}

// markdownSkillDescription is the report wording for skills and milestones,
// including the evidence trail that prevents scores from becoming context-free.
func markdownSkillDescription(skill SkillView) string {
	if skill.Kind == kindMilestone {
		if skill.Achieved && skill.Evidence != "" {
			return markdownTableCell(skill.StatusText) + " (" + markdownLink("evidence", skill.Evidence) + ")"
		}
		return markdownTableCell(skill.StatusText)
	}
	if !skill.HasData {
		return "untested"
	}
	description := fmt.Sprintf("%d", skill.Confidence)
	if tests := skill.EvidenceTestsText(); tests != "" {
		description += " (" + markdownTableCell(tests) + ")"
	}
	return description
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
