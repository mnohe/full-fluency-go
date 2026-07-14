package main

import (
	"math"
	"slices"
)

const testsForFullVariety = 3

// confidenceTierWeight converts a validated attempt confidence tier into the
// multiplier used by the protocol's confidence formula.
func confidenceTierWeight(confidence string) float64 {
	switch confidence {
	case confidenceLow:
		return 0.6
	case confidenceMedium:
		return 0.8
	case confidenceHigh:
		return 1.0
	default:
		panic("invalid confidence tier reached confidence math: " + confidence)
	}
}

// testQuality summarizes how strongly a single test supports its referenced
// skills: the best score * confidence-tier-weight across all attempts.
func testQuality(attempts []Attempt) float64 {
	best := 0.0
	for _, attempt := range attempts {
		if quality := float64(attempt.Score) * confidenceTierWeight(attempt.Confidence); quality > best {
			best = quality
		}
	}
	return best
}

// skillConfidence aggregates every scorecard that lists slug in assesses or
// demonstrated. It also returns the contributing test IDs so the report can
// show the evidence behind each number.
func skillConfidence(slug string, cards []Scorecard) (confidence int, hasData bool, evidenceTests []string) {
	var qualities []float64
	for _, card := range cards {
		if len(card.Attempts) == 0 {
			continue
		}
		if !slices.Contains(card.Assesses, slug) && !slices.Contains(card.Demonstrated, slug) {
			continue
		}
		qualities = append(qualities, testQuality(card.Attempts))
		evidenceTests = append(evidenceTests, card.Name)
	}
	if len(qualities) == 0 {
		return 0, false, nil
	}

	sum := 0.0
	for _, quality := range qualities {
		sum += quality
	}
	average := sum / float64(len(qualities))

	variety := float64(len(qualities)) / testsForFullVariety
	if variety > 1 {
		variety = 1
	}

	return int(math.Round(average * variety)), true, evidenceTests
}
