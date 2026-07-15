package main

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// attemptDateLayout requires minute precision, not just a bare date. Two
// attempts landing on the same calendar date but in different tests need a
// real ordering signal -- otherwise the ladder falls back to directory
// iteration order, an incidental property, instead of what actually
// happened first (see confidence.go's date-ordered fold).
const attemptDateLayout = "2006-01-02T15:04"

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

// validateSkill checks one skills.yaml entry against the flat-skill model:
// every skill is independently scoreable regardless of category membership.
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
	validateRubric(v, path, skill.Rubric)
	if _, exists := index.levels[skill.Level]; !exists {
		v.addf("%s: level %q is not declared in levels", path, skill.Level)
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

// validateRubric enforces the tiered-rubric shape: Green is always required
// (every skill needs at least a top bar), and tiers fill from the bottom --
// a skill can't define Yellow without also defining Orange, since a rung
// with no written bar beneath it isn't reachable in any meaningful way.
func validateRubric(v *validator, path string, rubric Rubric) {
	if strings.TrimSpace(rubric.Green) == "" {
		v.addf("%s: rubric.green is required -- every skill needs at least a top bar", path)
	}
	if strings.TrimSpace(rubric.Yellow) != "" && strings.TrimSpace(rubric.Orange) == "" {
		v.addf("%s: rubric.yellow is set without rubric.orange -- tiers fill from the bottom, none can be skipped", path)
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

// validateScorecards checks every loaded test against the already-indexed
// registry. Missing metadata.yaml/scorecard.yaml pairs are handled by the
// loader so this function only sees tests that intentionally exist.
func validateScorecards(v *validator, sf SkillsFile, index registryIndex, cards []Scorecard) {
	for _, card := range cards {
		validateScorecard(v, sf, index, card)
	}
}

// validateScorecard enforces the metadata/scorecard schema plus derived
// invariants: the directory name, metadata.yaml's name, and scorecard.yaml's
// own test_id must all agree, and "passed" must match the attempt history.
func validateScorecard(v *validator, sf SkillsFile, index registryIndex, card Scorecard) {
	if !validSlug(card.DirName) {
		v.addf("tests/%s: directory name must match %s", card.DirName, slugPattern.String())
	}
	if strings.TrimSpace(card.Name) == "" {
		v.addf("%s: name is required", card.MetaPath)
	} else if card.Name != card.DirName {
		v.addf("%s: name %q must match directory %q", card.MetaPath, card.Name, card.DirName)
	}
	if strings.TrimSpace(card.Summary) == "" {
		v.addf("%s: summary is required", card.MetaPath)
	}
	if len(card.Assesses) == 0 {
		v.addf("%s: assesses must contain at least one skill slug", card.MetaPath)
	}

	if strings.TrimSpace(card.TestID) == "" {
		v.addf("%s: test_id is required", card.ScorePath)
	} else if card.TestID != card.DirName {
		v.addf("%s: test_id %q must match directory %q", card.ScorePath, card.TestID, card.DirName)
	}

	seenEvidenceSlugs := map[string]string{}
	validateEvidenceSlugs(v, sf, index, card.MetaPath, "assesses", card.Assesses, seenEvidenceSlugs)
	validateEvidenceSlugs(v, sf, index, card.ScorePath, "demonstrated", card.Demonstrated, seenEvidenceSlugs)

	relevantSlugs := map[string]struct{}{}
	for _, slug := range card.Assesses {
		relevantSlugs[slug] = struct{}{}
	}
	for _, slug := range card.Demonstrated {
		relevantSlugs[slug] = struct{}{}
	}

	for i, attempt := range card.Attempts {
		validateAttempt(v, fmt.Sprintf("%s: attempts[%d]", card.ScorePath, i), attempt, index, relevantSlugs)
	}
	expectedPassed := scorecardPassed(card.Attempts)
	if card.Passed != expectedPassed {
		v.addf("%s: passed is %t, want %t based on attempts", card.ScorePath, card.Passed, expectedPassed)
	}
}

// validateEvidenceSlugs checks assesses/demonstrated lists. A slug may appear
// once across both lists and must name a real skill, never a category.
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
		if _, exists := index.skills[slug]; !exists {
			if _, isCategory := sf.Categories[slug]; isCategory {
				v.addf("%s references category %q; scorecards must reference skill slugs", fieldPath, slug)
				continue
			}
			v.addf("%s references unknown skill slug %q", fieldPath, slug)
		}
	}
}

// validateAttempt checks the values that feed the ladder algorithm. Keeping
// invalid tiers out here lets confidence.go stay pure state transitions with
// no defensive input-checking of its own.
//
// index and relevantSlugs let this check two things a bare enum check
// can't: that every skill graded here is actually one this test assesses or
// demonstrates (relevantSlugs), and that the tier graded is one the skill's
// own rubric actually defines (index) -- a green-only skill can never be
// handed tier: orange, because there's no written orange bar for it to mean
// anything.
func validateAttempt(v *validator, path string, attempt Attempt, index registryIndex, relevantSlugs map[string]struct{}) {
	if _, err := time.Parse(attemptDateLayout, attempt.Date); err != nil {
		v.addf("%s: date %q must use YYYY-MM-DDTHH:MM", path, attempt.Date)
	}
	if len(attempt.Tiers) == 0 {
		v.addf("%s: tiers must grade at least one skill", path)
	}

	seenSkills := map[string]struct{}{}
	allGreen := true
	for i, st := range attempt.Tiers {
		tierPath := fmt.Sprintf("%s: tiers[%d]", path, i)
		if strings.TrimSpace(st.Skill) == "" {
			v.addf("%s: skill is required", tierPath)
		} else if _, duplicate := seenSkills[st.Skill]; duplicate {
			v.addf("%s: duplicate skill %q", tierPath, st.Skill)
		} else {
			seenSkills[st.Skill] = struct{}{}
			if _, relevant := relevantSlugs[st.Skill]; !relevant {
				v.addf("%s: skill %q is not in this test's assesses/demonstrated", tierPath, st.Skill)
			}
		}

		if !validTier(st.Tier) {
			v.addf("%s: tier %q must be %q, %q, %q, or %q -- graded directly against the skill's rubric, no free-floating confidence guess", tierPath, st.Tier, tierFail, stateOrange, stateYellow, stateGreen)
		} else if skill, known := index.skills[st.Skill]; known && !rubricDefinesTier(skill.Rubric, st.Tier) {
			v.addf("%s: tier %q is not defined in %q's rubric", tierPath, st.Tier, st.Skill)
		}

		if st.Tier != stateGreen {
			allGreen = false
		}
	}

	if strings.TrimSpace(attempt.Notes) == "" && !allGreen {
		v.addf("%s: notes is required unless every graded skill is a clean green-tier pass -- a fail or a lower-tier pass has something worth explaining", path)
	}
}

// rubricDefinesTier reports whether a skill's rubric actually has a written
// bar for the given tier. tierFail always counts -- it just means the
// attempt didn't clear the lowest bar the skill defines, which is always a
// meaningful outcome regardless of which tiers exist.
func rubricDefinesTier(r Rubric, tier string) bool {
	switch tier {
	case tierFail:
		return true
	case stateOrange:
		return strings.TrimSpace(r.Orange) != ""
	case stateYellow:
		return strings.TrimSpace(r.Yellow) != ""
	case stateGreen:
		return strings.TrimSpace(r.Green) != ""
	default:
		return false
	}
}

// scorecardPassed is the protocol's test-level pass rule: did any attempt
// clear at least the lowest defined tier for at least one graded skill. It
// still matters enormously which tier a pass reached for what it proves
// about each *skill* (see confidence.go), but a test that was cleared, even
// narrowly, was cleared.
func scorecardPassed(attempts []Attempt) bool {
	for _, attempt := range attempts {
		for _, st := range attempt.Tiers {
			if st.Tier != tierFail {
				return true
			}
		}
	}
	return false
}

// validTier is the enum check for an attempt's graded tier.
func validTier(tier string) bool {
	return tier == tierFail || tier == stateOrange || tier == stateYellow || tier == stateGreen
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
