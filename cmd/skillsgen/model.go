package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	kindSkill     = "skill"
	kindMilestone = "milestone"

	confidenceLow    = "low"
	confidenceMedium = "medium"
	confidenceHigh   = "high"
)

// Category is display and rollup metadata only. A category is never scoreable
// on its own and scorecards must never reference category slugs.
type Category struct {
	Label string `yaml:"label"`
}

// Skill is one flat registry entry from skills.yaml.
//
// kind: skill entries derive confidence from scorecards.
// kind: milestone entries are one-time achievements edited in skills.yaml.
type Skill struct {
	Slug       string   `yaml:"slug"`
	Label      string   `yaml:"label"`
	Level      string   `yaml:"level"`
	Kind       string   `yaml:"kind"`
	Categories []string `yaml:"categories,omitempty"`
	Achieved   bool     `yaml:"achieved,omitempty"`
	Evidence   string   `yaml:"evidence,omitempty"`
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

// Attempt is one sitting of one assessment. Date remains a string in the raw
// model so the YAML stays simple; validation parses it as YYYY-MM-DD.
type Attempt struct {
	Date       string `yaml:"date"`
	Score      int    `yaml:"score"`
	Confidence string `yaml:"confidence"`
	Notes      string `yaml:"notes"`
}

// Scorecard is tests/<test_id>/scorecard.yaml plus non-YAML source metadata
// used for precise validation errors.
type Scorecard struct {
	Name         string    `yaml:"name"`
	Summary      string    `yaml:"summary"`
	Assesses     []string  `yaml:"assesses"`
	Demonstrated []string  `yaml:"demonstrated"`
	Passed       bool      `yaml:"passed"`
	Attempts     []Attempt `yaml:"attempts"`

	Path   string `yaml:"-"`
	TestID string `yaml:"-"`
}

// loadScorecards walks the first level of testsDir in lexical order (the order
// os.ReadDir guarantees) so generated evidence trails remain deterministic.
func loadScorecards(testsDir string) ([]Scorecard, error) {
	entries, err := os.ReadDir(testsDir)
	if err != nil {
		return nil, fmt.Errorf("read tests directory %s: %w", testsDir, err)
	}

	var cards []Scorecard
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(testsDir, e.Name(), "scorecard.yaml")
		var sc Scorecard
		if err := decodeYAMLFile(path, &sc); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return nil, err
		}
		sc.Path = path
		sc.TestID = e.Name()
		cards = append(cards, sc)
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
