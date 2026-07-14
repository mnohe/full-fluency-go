package main

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const passingScore = 80

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validationError reports every problem found in one pass. A generator should
// not make users fix data one typo at a time when all invariants are knowable.
type validationError []string

func (e validationError) Error() string {
	var b strings.Builder
	b.WriteString("validation failed:")
	for _, problem := range e {
		b.WriteString("\n- ")
		b.WriteString(problem)
	}
	return b.String()
}

// validator accumulates protocol violations while keeping call sites concise.
type validator struct {
	problems []string
}

func (v *validator) addf(format string, args ...any) {
	v.problems = append(v.problems, fmt.Sprintf(format, args...))
}

func (v *validator) err() error {
	if len(v.problems) == 0 {
		return nil
	}
	return validationError(v.problems)
}

// validateInputs is the executable form of PROTOCOL.adoc's data contract.
// Rendering only starts after this succeeds, so later code can stay focused on
// transformation instead of repeatedly defending against malformed inputs.
func validateInputs(sf SkillsFile, cards []Scorecard) error {
	var v validator
	index := validateSkillsFile(&v, sf)
	validateScorecards(&v, sf, index, cards)
	return v.err()
}

// registryIndex is the validated lookup surface the scorecard checks need.
// Building it while validating skills keeps later checks O(1) and avoids
// repeatedly scanning the registry for every scorecard slug.
type registryIndex struct {
	levels     map[string]struct{}
	categories map[string]struct{}
	skills     map[string]Skill
}

// validateSkillsFile checks the registry itself and returns the lookup maps
// needed to validate scorecards against that registry.
func validateSkillsFile(v *validator, sf SkillsFile) registryIndex {
	index := registryIndex{
		levels:     map[string]struct{}{},
		categories: map[string]struct{}{},
		skills:     map[string]Skill{},
	}

	if len(sf.Levels) == 0 {
		v.addf("skills.yaml: levels must contain at least one level")
	}
	for i, level := range sf.Levels {
		if strings.TrimSpace(level) == "" {
			v.addf("skills.yaml: levels[%d] is empty", i)
			continue
		}
		if _, exists := index.levels[level]; exists {
			v.addf("skills.yaml: duplicate level %q", level)
			continue
		}
		index.levels[level] = struct{}{}
	}

	categorySlugs := sortedCategorySlugs(sf.Categories)
	for _, slug := range categorySlugs {
		category := sf.Categories[slug]
		if !validSlug(slug) {
			v.addf("skills.yaml: category slug %q must match %s", slug, slugPattern.String())
		}
		if strings.TrimSpace(category.Label) == "" {
			v.addf("skills.yaml: category %q has empty label", slug)
		}
		index.categories[slug] = struct{}{}
	}

	if len(sf.Skills) == 0 {
		v.addf("skills.yaml: skills must contain at least one entry")
	}
	for i, skill := range sf.Skills {
		path := fmt.Sprintf("skills.yaml: skills[%d]", i)
		validateSkill(v, path, skill, index)
		if skill.Slug != "" {
			index.skills[skill.Slug] = skill
		}
	}

	for i, ref := range sf.References {
		validateReference(v, fmt.Sprintf("skills.yaml: references[%d]", i), ref)
	}

	return index
}

// validateSkill checks one skills.yaml entry against the flat-skill model.
// It intentionally treats categories as rollup metadata, not as type names or
// namespace qualifiers.
func validateSkill(v *validator, path string, skill Skill, index registryIndex) {
	if strings.TrimSpace(skill.Slug) == "" {
		v.addf("%s: slug is required", path)
	} else if !validSlug(skill.Slug) {
		v.addf("%s: slug %q must match %s", path, skill.Slug, slugPattern.String())
	} else if _, exists := index.skills[skill.Slug]; exists {
		v.addf("%s: duplicate skill slug %q", path, skill.Slug)
	}

	if strings.TrimSpace(skill.Label) == "" {
		v.addf("%s: label is required", path)
	}
	if _, exists := index.levels[skill.Level]; !exists {
		v.addf("%s: level %q is not declared in levels", path, skill.Level)
	}

	switch skill.Kind {
	case kindSkill:
		if skill.Evidence != "" {
			v.addf("%s: skill entries must not set evidence", path)
		}
	case kindMilestone:
		if len(skill.Categories) != 0 {
			v.addf("%s: milestones must not belong to confidence rollup categories", path)
		}
		if skill.Achieved && strings.TrimSpace(skill.Evidence) == "" {
			v.addf("%s: achieved milestones must include evidence", path)
		}
		if skill.Evidence != "" {
			validateURL(v, path+": evidence", skill.Evidence)
		}
	default:
		v.addf("%s: kind %q must be %q or %q", path, skill.Kind, kindSkill, kindMilestone)
	}

	seenCategories := map[string]struct{}{}
	for _, categorySlug := range skill.Categories {
		if _, duplicate := seenCategories[categorySlug]; duplicate {
			v.addf("%s: duplicate category %q", path, categorySlug)
			continue
		}
		seenCategories[categorySlug] = struct{}{}
		if _, exists := index.categories[categorySlug]; !exists {
			v.addf("%s: category %q is not declared", path, categorySlug)
		}
	}
}

// validateReference keeps bibliography entries useful without requiring every
// skill to name a precise citation.
func validateReference(v *validator, path string, ref Reference) {
	if strings.TrimSpace(ref.Name) == "" {
		v.addf("%s: name is required", path)
	}
	if ref.URL != "" {
		validateURL(v, path+": url", ref.URL)
	}
	if len(ref.Topics) == 0 {
		v.addf("%s: topics must contain at least one topic", path)
	}
	for i, topic := range ref.Topics {
		if strings.TrimSpace(topic) == "" {
			v.addf("%s: topics[%d] is empty", path, i)
		}
	}
}

// validateScorecards checks every loaded scorecard against the already-indexed
// registry. Missing scorecard.yaml files are handled by the loader so this
// function only sees scorecards that intentionally exist.
func validateScorecards(v *validator, sf SkillsFile, index registryIndex, cards []Scorecard) {
	for _, card := range cards {
		path := card.Path
		if path == "" {
			path = fmt.Sprintf("scorecard %q", card.Name)
		}
		validateScorecard(v, sf, index, path, card)
	}
}

// validateScorecard enforces the scorecard schema plus derived invariants such
// as "passed" matching the attempt history.
func validateScorecard(v *validator, sf SkillsFile, index registryIndex, path string, card Scorecard) {
	if strings.TrimSpace(card.Name) == "" {
		v.addf("%s: name is required", path)
	} else if card.TestID != "" && card.Name != card.TestID {
		v.addf("%s: name %q must match directory %q", path, card.Name, card.TestID)
	}
	if strings.TrimSpace(card.Summary) == "" {
		v.addf("%s: summary is required", path)
	}
	if len(card.Assesses) == 0 {
		v.addf("%s: assesses must contain at least one skill slug", path)
	}

	seenEvidenceSlugs := map[string]string{}
	validateEvidenceSlugs(v, sf, index, path, "assesses", card.Assesses, seenEvidenceSlugs)
	validateEvidenceSlugs(v, sf, index, path, "demonstrated", card.Demonstrated, seenEvidenceSlugs)

	for i, attempt := range card.Attempts {
		validateAttempt(v, fmt.Sprintf("%s: attempts[%d]", path, i), attempt)
	}
	expectedPassed := scorecardPassed(card.Attempts)
	if card.Passed != expectedPassed {
		v.addf("%s: passed is %t, want %t based on attempts", path, card.Passed, expectedPassed)
	}
}

// validateEvidenceSlugs checks assesses/demonstrated lists. A slug may appear
// once across both lists, must name a skill, and must not name a category or
// milestone.
func validateEvidenceSlugs(v *validator, sf SkillsFile, index registryIndex, path, field string, slugs []string, seen map[string]string) {
	for i, slug := range slugs {
		fieldPath := fmt.Sprintf("%s: %s[%d]", path, field, i)
		if strings.TrimSpace(slug) == "" {
			v.addf("%s is empty", fieldPath)
			continue
		}
		if previousField, duplicate := seen[slug]; duplicate {
			v.addf("%s duplicates slug %q already listed in %s", fieldPath, slug, previousField)
			continue
		}
		seen[slug] = field
		skill, exists := index.skills[slug]
		if !exists {
			if _, isCategory := sf.Categories[slug]; isCategory {
				v.addf("%s references category %q; scorecards must reference skill slugs", fieldPath, slug)
				continue
			}
			v.addf("%s references unknown skill slug %q", fieldPath, slug)
			continue
		}
		if skill.Kind == kindMilestone {
			v.addf("%s references milestone %q; milestones are not scored by tests", fieldPath, slug)
		}
	}
}

// validateAttempt checks the values that feed the confidence algorithm. Keeping
// invalid numbers and tiers out here lets confidence.go stay pure arithmetic.
func validateAttempt(v *validator, path string, attempt Attempt) {
	if _, err := time.Parse(time.DateOnly, attempt.Date); err != nil {
		v.addf("%s: date %q must use YYYY-MM-DD", path, attempt.Date)
	}
	if attempt.Score < 0 || attempt.Score > 100 {
		v.addf("%s: score %d must be between 0 and 100", path, attempt.Score)
	}
	if !validConfidence(attempt.Confidence) {
		v.addf("%s: confidence %q must be %q, %q, or %q", path, attempt.Confidence, confidenceLow, confidenceMedium, confidenceHigh)
	}
}

// scorecardPassed is the protocol's pass rule in executable form.
func scorecardPassed(attempts []Attempt) bool {
	for _, attempt := range attempts {
		if attempt.Score >= passingScore && confidencePasses(attempt.Confidence) {
			return true
		}
	}
	return false
}

// confidencePasses captures the "medium or high" half of the pass rule.
func confidencePasses(confidence string) bool {
	return confidence == confidenceMedium || confidence == confidenceHigh
}

// validConfidence is the enum check for attempt confidence.
func validConfidence(confidence string) bool {
	return confidence == confidenceLow || confidence == confidenceMedium || confidence == confidenceHigh
}

// validSlug keeps skill, category, and test-reference slugs shell- and
// URL-friendly without introducing a second naming convention.
func validSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}

// validateURL accepts only absolute URLs because evidence and references should
// remain meaningful when README.md is viewed outside this checkout.
func validateURL(v *validator, path, raw string) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		v.addf("%s %q must be an absolute URL", path, raw)
	}
}

// sortedCategorySlugs makes validation diagnostics deterministic even though
// Go map iteration order is intentionally randomized.
func sortedCategorySlugs(categories map[string]Category) []string {
	slugs := make([]string, 0, len(categories))
	for slug := range categories {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}
