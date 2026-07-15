// Command reportgen renders the repository's generated Markdown progress
// report from skills.yaml and tests/*/scorecard.yaml. See PROTOCOL.adoc for
// the data contract this command enforces.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	defaultSkillsPath = "skills.yaml"
	defaultTestsDir   = "tests"
	defaultOutPath    = "README.md"
)

// main is deliberately tiny, keep it this way.
func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// config is the complete command-line contract after flag parsing.
// Keeping it as a value makes the rest of the pipeline independent from flag
// package globals and easy to construct in tests.
type config struct {
	skillsPath string
	testsDir   string
	outPath    string
}

// run owns the command workflow: parse flags, load inputs, validate the
// protocol invariants, build template-ready views, and write the single report.
func run(args []string, stdout, stderr io.Writer) error {
	cfg, err := parseConfig(args, stderr)
	if err != nil {
		return err
	}

	sf, err := loadSkills(cfg.skillsPath)
	if err != nil {
		return err
	}
	cards, err := loadScorecards(cfg.testsDir)
	if err != nil {
		return err
	}
	if err := validateInputs(sf, cards); err != nil {
		return err
	}

	testNumbers, testIndex := buildTestIndex(cards)
	report := reportData{
		Levels:     buildLevelViews(sf, cards, testNumbers),
		References: buildReferenceViews(sf.References),
		TestIndex:  testIndex,
	}
	if err := renderMarkdown(cfg.outPath, report); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s\n", cfg.outPath)
	return nil
}

// parseConfig uses a fresh FlagSet instead of flag.CommandLine so tests and
// repeated in-process invocations are isolated from one another.
func parseConfig(args []string, stderr io.Writer) (config, error) {
	cfg := config{
		skillsPath: defaultSkillsPath,
		testsDir:   defaultTestsDir,
		outPath:    defaultOutPath,
	}

	fs := flag.NewFlagSet("reportgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.skillsPath, "skills", cfg.skillsPath, "path to the skill registry")
	fs.StringVar(&cfg.testsDir, "tests", cfg.testsDir, "path to the tests directory")
	fs.StringVar(&cfg.outPath, "out", cfg.outPath, "output report path")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() != 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if strings.TrimSpace(cfg.outPath) == "" {
		return config{}, fmt.Errorf("--out must not be empty")
	}
	return cfg, nil
}
