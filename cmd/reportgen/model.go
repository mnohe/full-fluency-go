package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// tierFail marks an attempt that didn't clear even the lowest defined
// rubric bar. It is not a ladder rung (see confidence.go's stateNone vs.
// stateRed) -- it is purely an attempt-level grading outcome.
const tierFail = "fail"

// Category is display metadata only. A category is never scoreable on its
// own and scorecards must never reference category slugs.
type Category struct {
	Label string `yaml:"label"`
}

// Rubric is a skill's tiered pass bar: up to three one-sentence bars the
// agent grades an attempt against, from the top down. Green is required --
// every skill needs at least one bar. Orange and Yellow are optional: a
// skill that's genuinely binary (has it or doesn't, e.g. contribute_oss)
// defines only Green, and any pass on it climbs straight to green, the same
// way externally-verified attempts always have been logged at the top tier.
// A skill that defines Yellow must also define Orange -- tiers are filled
// from the bottom, never skipped.
//
// Splitting Orange/Yellow/Green out of a single sentence answers the
// question a flat rubric couldn't: a compound skill that seemed to need a
// caveat ("if/else, all three for-loop variants, AND switch, all in one
// program") was usually several different commands bundled together and
// should be split into separate skills (see "Skills are flat" in
// PROTOCOL.adoc) -- but a skill that's genuinely one command with a real
// depth gradient (the three `for` syntaxes, all still "loops") belongs in
// one skill, tiered.
type Rubric struct {
	Orange string `yaml:"orange,omitempty"`
	Yellow string `yaml:"yellow,omitempty"`
	Green  string `yaml:"green"`
}

// Skill is one flat, independently scoreable registry entry from skills.yaml.
// Every skill stands on its own regardless of category membership; there is
// no separate "milestone" kind -- a one-time, externally verifiable claim is
// just a skill whose evidence trail is a test whose attempt notes record
// what was checked, not a special schema shape.
type Skill struct {
	Slug       string   `yaml:"slug"`
	Label      string   `yaml:"label"`
	Rubric     Rubric   `yaml:"rubric"`
	Level      string   `yaml:"level"`
	Categories []string `yaml:"categories,omitempty"`
}

// Reference is one bibliography entry. Topics are metadata that explain why
// the reference belongs in the registry; rendering lists each reference once.
type Reference struct {
	Name    string   `yaml:"name"`
	Authors []string `yaml:"authors,omitempty"`
	URL     string   `yaml:"url,omitempty"`
	Topics  []string `yaml:"topics"`
}

// SkillsFile is the complete skills.yaml schema.
type SkillsFile struct {
	Levels     []string            `yaml:"levels"`
	References []Reference         `yaml:"references"`
	Categories map[string]Category `yaml:"categories"`
	Skills     []Skill             `yaml:"skills"`
}

// loadSkills reads skills.yaml with strict field checking. Unknown YAML fields
// are almost always typos, so accepting them would make the generated report
// less trustworthy than the source data appears to be.
func loadSkills(path string) (SkillsFile, error) {
	var sf SkillsFile
	if err := decodeYAMLFile(path, &sf); err != nil {
		return sf, err
	}
	return sf, nil
}

// SkillTier is one skill's graded tier within a single Attempt. A test can
// assess or demonstrate more than one skill at once (see "Sizing a test" in
// PROTOCOL.adoc), and different skills touched by the same sitting don't
// necessarily clear the same bar -- so the tier is graded per skill, not
// once for the whole attempt.
type SkillTier struct {
	Skill string `yaml:"skill"`
	Tier  string `yaml:"tier"`
}

// Attempt is one sitting of one assessment. Date remains a string in the raw
// model so the YAML stays simple; validation parses it as
// YYYY-MM-DDTHH:MM, so that same-day retakes across different tests still
// sort into their real order instead of an incidental one (see
// validateAttempt).
//
// Tiers holds one entry per skill this sitting actually graded, each graded
// directly against that skill's own Rubric, top-down: which bar (Green,
// then Yellow, then Orange) did this attempt clear for that skill, or did
// it clear none of them (tierFail). A tier used to be one binary pass/fail
// plus a separately self-assessed confidence tier, collapsed into one field
// per skill, because once the rubric itself spells out what
// orange/yellow/green each require, there's nothing left for a
// free-floating confidence judgment to add: the bar cleared *is* the rigor.
// Each skill's tier is what determines which rung of that skill's ladder
// the attempt lands on (see confidence.go).
//
// Notes is required unless every skill in Tiers cleared Green: a fail or a
// lower-tier pass on any graded skill has something worth explaining, while
// an attempt where everything cleared the top bar already speaks for
// itself (see validateAttempt).
type Attempt struct {
	Date  string      `yaml:"date"`
	Tiers []SkillTier `yaml:"tiers"`
	Notes string      `yaml:"notes"`
}

// TestMetadata is tests/<test_id>/metadata.yaml, the read-only test spec:
// what the test checks for and which skills it was designed to evaluate.
// It isn't edited once a test has attempts recorded against it -- redesign
// as a new test instead of changing what an existing one measures.
type TestMetadata struct {
	Name     string   `yaml:"name"`
	Summary  string   `yaml:"summary"`
	Assesses []string `yaml:"assesses"`
}

// TestResults is tests/<test_id>/scorecard.yaml, the mutable attempt history
// for one test: does it carry its own test_id (checked against the metadata
// it's paired with and the directory it lives in) so the file is
// self-identifying even read on its own.
type TestResults struct {
	TestID       string    `yaml:"test_id"`
	Passed       bool      `yaml:"passed"`
	Demonstrated []string  `yaml:"demonstrated"`
	Attempts     []Attempt `yaml:"attempts"`
}

// Scorecard merges one test's TestMetadata and TestResults into the single
// shape confidence math and rendering work with. Path fields are load-time
// context for validation error messages, not YAML data.
type Scorecard struct {
	Name         string
	Summary      string
	Assesses     []string
	Demonstrated []string
	Passed       bool
	Attempts     []Attempt

	TestID    string // from scorecard.yaml's own test_id field
	DirName   string // the tests/<dir> this pair was loaded from
	MetaPath  string
	ScorePath string
}

// loadScorecards walks the first level of testsDir in lexical order (the
// order os.ReadDir guarantees) so generated evidence trails remain
// deterministic. A missing testsDir is treated as zero tests, since
// tests/ doesn't exist until the first test is created. A directory with
// neither file is skipped (not yet a test); a directory with only one of
// the two is a load error, since a lone metadata.yaml or scorecard.yaml is
// always a mistake, never a valid state.
func loadScorecards(testsDir string) ([]Scorecard, error) {
	entries, err := os.ReadDir(testsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tests directory %s: %w", testsDir, err)
	}

	var cards []Scorecard
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(testsDir, e.Name())
		metaPath := filepath.Join(dir, "metadata.yaml")
		scorePath := filepath.Join(dir, "scorecard.yaml")

		var meta TestMetadata
		metaErr := decodeYAMLFile(metaPath, &meta)
		var results TestResults
		resultsErr := decodeYAMLFile(scorePath, &results)

		switch {
		case errors.Is(metaErr, os.ErrNotExist) && errors.Is(resultsErr, os.ErrNotExist):
			continue
		case errors.Is(metaErr, os.ErrNotExist):
			return nil, fmt.Errorf("%s: scorecard.yaml exists but metadata.yaml is missing", dir)
		case errors.Is(resultsErr, os.ErrNotExist):
			return nil, fmt.Errorf("%s: metadata.yaml exists but scorecard.yaml is missing", dir)
		case metaErr != nil:
			return nil, metaErr
		case resultsErr != nil:
			return nil, resultsErr
		}

		cards = append(cards, Scorecard{
			Name:         meta.Name,
			Summary:      meta.Summary,
			Assesses:     meta.Assesses,
			Demonstrated: results.Demonstrated,
			Passed:       results.Passed,
			Attempts:     results.Attempts,
			TestID:       results.TestID,
			DirName:      e.Name(),
			MetaPath:     metaPath,
			ScorePath:    scorePath,
		})
	}
	return cards, nil
}

// decodeYAMLFile is the one YAML door into the program. Centralizing strict
// decoding keeps every input file under the same "unknown fields are errors"
// rule.
func decodeYAMLFile(path string, dst any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
