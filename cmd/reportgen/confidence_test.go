package main

import "testing"

func newCard(name string, assesses, demonstrated []string, attempts ...Attempt) Scorecard {
	return Scorecard{Name: name, Assesses: assesses, Demonstrated: demonstrated, Attempts: attempts}
}

func at(skill, date, tier string) Attempt {
	return Attempt{Date: date, Tiers: []SkillTier{{Skill: skill, Tier: tier}}, Notes: "n/a"}
}

func TestSkillLadderNoEvidenceIsUntested(t *testing.T) {
	cards := []Scorecard{newCard("unrelated", []string{"other_skill"}, nil, at("other_skill", "2026-01-01T09:00", stateGreen))}
	state, hasData, evidence := skillLadder("read_basic_code", cards)
	if state != stateNone || hasData || evidence != nil {
		t.Fatalf("skillLadder() = (%q, %t, %#v), want (%q, false, nil)", state, hasData, evidence, stateNone)
	}
}

func TestSkillLadderFirstFailStaysUntested(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"read_basic_code"}, nil, at("read_basic_code", "2026-01-01T09:00", tierFail))}
	state, hasData, _ := skillLadder("read_basic_code", cards)
	if state != stateNone {
		t.Fatalf("state = %q, want %q (a first-ever fail should not land on red)", state, stateNone)
	}
	if !hasData {
		t.Fatal("hasData = false, want true (an attempt was made)")
	}
}

func TestSkillLadderClimbsToTierCleared(t *testing.T) {
	for _, tc := range []struct {
		tier string
		want string
	}{
		{stateOrange, stateOrange},
		{stateYellow, stateYellow},
		{stateGreen, stateGreen},
	} {
		cards := []Scorecard{newCard("t1", []string{"s"}, nil, at("s", "2026-01-01T09:00", tc.tier))}
		state, _, _ := skillLadder("s", cards)
		if state != tc.want {
			t.Errorf("tier %q -> state %q, want %q", tc.tier, state, tc.want)
		}
	}
}

func TestSkillLadderWeakerPassIsNoOpNotDemotion(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"s"}, nil,
		at("s", "2026-01-01T09:00", stateGreen),
		at("s", "2026-01-02T09:00", stateOrange),
	)}
	state, _, _ := skillLadder("s", cards)
	if state != stateGreen {
		t.Fatalf("state = %q, want %q (a weaker later pass should not demote)", state, stateGreen)
	}
}

func TestSkillLadderFailStepsDownOneRungAndFloorsAtRed(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"s"}, nil,
		at("s", "2026-01-01T09:00", stateGreen), // green
		at("s", "2026-01-02T09:00", tierFail),   // -> yellow
		at("s", "2026-01-03T09:00", tierFail),   // -> orange
		at("s", "2026-01-04T09:00", tierFail),   // -> red
		at("s", "2026-01-05T09:00", tierFail),   // holds red
	)}
	state, _, _ := skillLadder("s", cards)
	if state != stateRed {
		t.Fatalf("state = %q, want %q", state, stateRed)
	}
}

func TestSkillLadderRecoveryFromRedLandsAtTheNewAttemptsTier(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"s"}, nil,
		at("s", "2026-01-01T09:00", stateGreen),  // green
		at("s", "2026-01-02T09:00", tierFail),    // yellow
		at("s", "2026-01-03T09:00", tierFail),    // orange
		at("s", "2026-01-04T09:00", tierFail),    // red
		at("s", "2026-01-05T09:00", stateOrange), // recovers to orange, not back to green
	)}
	state, _, _ := skillLadder("s", cards)
	if state != stateOrange {
		t.Fatalf("state = %q, want %q (recovery should not restore the prior rung)", state, stateOrange)
	}
}

func TestSkillLadderThreeGreensEarnMaster(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"s"}, nil,
		at("s", "2026-01-01T09:00", stateGreen),
		at("s", "2026-01-02T09:00", stateGreen),
		at("s", "2026-01-03T09:00", stateGreen),
	)}
	state, _, _ := skillLadder("s", cards)
	if state != stateMaster {
		t.Fatalf("state = %q, want %q", state, stateMaster)
	}
}

func TestSkillLadderMasterStreakResetsOnFail(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"s"}, nil,
		at("s", "2026-01-01T09:00", stateGreen),
		at("s", "2026-01-02T09:00", stateGreen),
		at("s", "2026-01-03T09:00", tierFail),   // resets the streak, drops to yellow
		at("s", "2026-01-04T09:00", stateGreen), // yellow -> green, streak restarts at 1
		at("s", "2026-01-05T09:00", stateGreen),
	)}
	state, _, _ := skillLadder("s", cards)
	if state != stateGreen {
		t.Fatalf("state = %q, want %q (streak should have reset, not carried over the fail)", state, stateGreen)
	}
}

func TestSkillLadderFailAtMasterDropsToGreen(t *testing.T) {
	cards := []Scorecard{newCard("t1", []string{"s"}, nil,
		at("s", "2026-01-01T09:00", stateGreen),
		at("s", "2026-01-02T09:00", stateGreen),
		at("s", "2026-01-03T09:00", stateGreen), // master
		at("s", "2026-01-04T09:00", tierFail),
	)}
	state, _, _ := skillLadder("s", cards)
	if state != stateGreen {
		t.Fatalf("state = %q, want %q", state, stateGreen)
	}
}

func TestSkillLadderDemonstratedOnlyCountsAtGreenTier(t *testing.T) {
	cards := []Scorecard{newCard("t1", nil, []string{"s"}, at("s", "2026-01-01T09:00", stateOrange))}
	state, hasData, evidence := skillLadder("s", cards)
	if state != stateNone || hasData {
		t.Fatalf("state = %q hasData=%t, want %q/false (an orange-tier demonstrated pass must not count)", state, hasData, stateNone)
	}
	if evidence != nil {
		t.Fatalf("evidence = %#v, want nil", evidence)
	}
}

func TestSkillLadderDemonstratedGreenTierClimbsToGreen(t *testing.T) {
	cards := []Scorecard{newCard("t1", nil, []string{"s"}, at("s", "2026-01-01T09:00", stateGreen))}
	state, hasData, _ := skillLadder("s", cards)
	if !hasData || state != stateGreen {
		t.Fatalf("state = %q hasData=%t, want %q/true", state, hasData, stateGreen)
	}
}

func TestSkillLadderDemonstratedFailStillCountsAsASlip(t *testing.T) {
	cards := []Scorecard{
		newCard("t1", []string{"s"}, nil, at("s", "2026-01-01T09:00", stateGreen)),
		newCard("t2", nil, []string{"s"}, at("s", "2026-01-02T09:00", tierFail)),
	}
	state, _, _ := skillLadder("s", cards)
	if state != stateYellow {
		t.Fatalf("state = %q, want %q (a demonstrated fail should still step down)", state, stateYellow)
	}
}

func TestSkillLadderEventsAcrossTestsAreOrderedByDate(t *testing.T) {
	cards := []Scorecard{
		newCard("later", []string{"s"}, nil, at("s", "2026-01-05T09:00", tierFail)),
		newCard("earlier", []string{"s"}, nil, at("s", "2026-01-01T09:00", stateGreen)),
	}
	state, _, evidence := skillLadder("s", cards)
	if state != stateYellow {
		t.Fatalf("state = %q, want %q (pass then fail, in date order, regardless of card iteration order)", state, stateYellow)
	}
	if len(evidence) != 2 {
		t.Fatalf("evidence = %#v, want 2 entries", evidence)
	}
}

func TestSkillLadderSameDayEventsOrderByTimeNotCardIterationOrder(t *testing.T) {
	// Both attempts fall on the same calendar date. "zzz_later" sorts after
	// "aaa_earlier" alphabetically (the opposite of when they actually
	// happened), so if the ladder fell back to card/directory order instead
	// of the timestamp, it would fold the fail before the pass and land on
	// green instead of yellow.
	cards := []Scorecard{
		newCard("zzz_later", []string{"s"}, nil, at("s", "2026-01-01T16:00", tierFail)),
		newCard("aaa_earlier", []string{"s"}, nil, at("s", "2026-01-01T09:00", stateGreen)),
	}
	state, _, _ := skillLadder("s", cards)
	if state != stateYellow {
		t.Fatalf("state = %q, want %q (same-day events must order by timestamp, not card order)", state, stateYellow)
	}
}

func TestSkillLadderIgnoresAttemptlessTests(t *testing.T) {
	cards := []Scorecard{newCard("planned", []string{"s"}, nil)}
	state, hasData, evidence := skillLadder("s", cards)
	if state != stateNone || hasData || evidence != nil {
		t.Fatalf("skillLadder() = (%q, %t, %#v), want (%q, false, nil)", state, hasData, evidence, stateNone)
	}
}

func TestSkillLadderGradesEachSkillInAnAttemptIndependently(t *testing.T) {
	// One attempt, one sitting, two skills -- skill_a clears green, skill_b
	// only clears orange. Neither skill's tier should leak onto the other.
	attempt := Attempt{
		Date: "2026-01-01T09:00",
		Tiers: []SkillTier{
			{Skill: "skill_a", Tier: stateGreen},
			{Skill: "skill_b", Tier: stateOrange},
		},
		Notes: "n/a",
	}
	cards := []Scorecard{newCard("t1", []string{"skill_a", "skill_b"}, nil, attempt)}

	stateA, _, _ := skillLadder("skill_a", cards)
	if stateA != stateGreen {
		t.Fatalf("skill_a state = %q, want %q", stateA, stateGreen)
	}
	stateB, _, _ := skillLadder("skill_b", cards)
	if stateB != stateOrange {
		t.Fatalf("skill_b state = %q, want %q (must not inherit skill_a's green)", stateB, stateOrange)
	}
}

func TestSkillLadderIgnoresAttemptsThatDidNotGradeThisSkill(t *testing.T) {
	// The test assesses both skills, but this particular attempt only
	// graded skill_a. skill_b should stay untested, not "fail".
	attempt := Attempt{
		Date:  "2026-01-01T09:00",
		Tiers: []SkillTier{{Skill: "skill_a", Tier: stateGreen}},
		Notes: "n/a",
	}
	cards := []Scorecard{newCard("t1", []string{"skill_a", "skill_b"}, nil, attempt)}

	stateB, hasData, evidence := skillLadder("skill_b", cards)
	if stateB != stateNone || hasData || evidence != nil {
		t.Fatalf("skill_b: skillLadder() = (%q, %t, %#v), want (%q, false, nil)", stateB, hasData, evidence, stateNone)
	}
}
