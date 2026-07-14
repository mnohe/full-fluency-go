package main

import "testing"

func TestTestQualityUsesBestWeightedAttempt(t *testing.T) {
	attempts := []Attempt{
		{Score: 100, Confidence: confidenceLow},
		{Score: 80, Confidence: confidenceHigh},
		{Score: 90, Confidence: confidenceMedium},
	}

	if got, want := testQuality(attempts), 80.0; got != want {
		t.Fatalf("testQuality() = %v, want %v", got, want)
	}
}

func TestSkillConfidenceAppliesVarietyFactorAndEvidenceTrail(t *testing.T) {
	cards := []Scorecard{
		{Name: "alpha", Assesses: []string{"read_simple_code"}, Attempts: []Attempt{{Score: 90, Confidence: confidenceHigh}}},
		{Name: "beta", Demonstrated: []string{"read_simple_code"}, Attempts: []Attempt{{Score: 60, Confidence: confidenceHigh}}},
		{Name: "unrelated", Assesses: []string{"write_simple_functions"}, Attempts: []Attempt{{Score: 100, Confidence: confidenceHigh}}},
	}

	confidence, hasData, evidence := skillConfidence("read_simple_code", cards)
	if !hasData {
		t.Fatal("skillConfidence() reported no data")
	}
	if confidence != 50 {
		t.Fatalf("confidence = %d, want 50", confidence)
	}
	if got, want := evidence, []string{"alpha", "beta"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("evidence = %#v, want %#v", got, want)
	}
}

func TestSkillConfidenceHasNoDataWithoutAttempts(t *testing.T) {
	cards := []Scorecard{
		{Name: "empty", Assesses: []string{"read_simple_code"}},
	}

	confidence, hasData, evidence := skillConfidence("read_simple_code", cards)
	if confidence != 0 || hasData || evidence != nil {
		t.Fatalf("skillConfidence() = (%d, %t, %#v), want (0, false, nil)", confidence, hasData, evidence)
	}
}
