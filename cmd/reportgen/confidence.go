package main

import (
	"slices"
	"sort"
)

// The ladder a skill climbs and falls on. Ordered low to high; stateNone is
// a special case (never attempted) rather than the floor -- the floor for a
// skill that HAS been attempted is stateRed.
//
// stateOrange/stateYellow/stateGreen are deliberately the same strings an
// Attempt's Tier field uses (see model.go): a passing attempt's tier *is*
// the rung it climbs to, with no separate mapping step in between.
const (
	stateNone   = "none"
	stateRed    = colorRed
	stateOrange = colorOrange
	stateYellow = colorYellow
	stateGreen  = colorGreen
	stateMaster = colorGold
)

// rungOrder gives every reachable rung (everything except stateNone, which
// is never climbed to directly) a comparable ordinal.
var rungOrder = map[string]int{
	stateRed:    1,
	stateOrange: 2,
	stateYellow: 3,
	stateGreen:  4,
	stateMaster: 5,
}

// greensForMaster is how many green-tier passes, uninterrupted by a slip,
// it takes to earn the star -- counting the pass that reached green in the
// first place. Deliberately small and uniform across every skill and level:
// the one place a repeat count survives in this model, kept to a single
// tunable constant instead of a per-level or per-skill table.
const greensForMaster = 3

// stepDown is what one slip (a failed attempt) does to a skill's current
// rung. A skill that has never been proven (stateNone) does not turn red on
// a failed attempt -- red is a regression state, reachable only by falling
// from a rung the skill actually held. Once at red, further slips hold the
// floor rather than going anywhere else.
func stepDown(state string) string {
	switch state {
	case stateMaster:
		return stateGreen
	case stateGreen:
		return stateYellow
	case stateYellow:
		return stateOrange
	case stateOrange:
		return stateRed
	default:
		return state // stateRed and stateNone both hold
	}
}

// ladderEvent is one dated, sourced touch of a skill, gathered from every
// attempt in every test that assesses or demonstrates it.
type ladderEvent struct {
	date         string
	tier         string // tierFail, or the rung (stateOrange/stateYellow/stateGreen) this attempt cleared
	demonstrated bool   // true if this card only demonstrated the skill, never asserted it
}

// attemptTier looks up whether a is one of the sittings that actually
// graded slug, and if so, which tier it cleared for that skill specifically
// -- a single attempt can grade several different skills at once, each at
// its own tier (see SkillTier in model.go), so this never assumes the
// attempt's tiers are uniform across every skill it touches.
func attemptTier(a Attempt, slug string) (tier string, graded bool) {
	for _, st := range a.Tiers {
		if st.Skill == slug {
			return st.Tier, true
		}
	}
	return "", false
}

// skillLadder walks every attempt that touched slug, in date order, and
// folds them through the climb/slip rules to the skill's current state. No
// score is averaged and no fixed repeat count gates ordinary progress: a
// skill either has evidence or it doesn't, and each attempt's tier for that
// specific skill -- graded directly against the skill's own Rubric -- says
// exactly where that evidence puts it.
//
// demonstrated-only evidence is held to a higher bar than assessed evidence
// before it counts at all: only a green-tier demonstrated pass climbs
// anything (straight to green), so an incidental, lightly-tested touch on a
// skill can't hand out a rating the way a deliberately designed test can. An
// orange- or yellow-tier demonstrated pass is invisible to the ladder --
// neither a climb nor a slip. This does mean a skill can reach green via a
// single lucky incidental demonstration and be walked back down by
// subsequent evidence; that's accepted as a known, self-correcting risk
// rather than guarded against with more machinery.
//
// evidenceTests reports every test that touched the skill at all (assessed
// or demonstrated, whether or not it moved the ladder) so the report's
// Evidence column can show the full trail.
func skillLadder(slug string, cards []Scorecard) (state string, hasData bool, evidenceTests []string) {
	var events []ladderEvent
	for _, card := range cards {
		assesses := slices.Contains(card.Assesses, slug)
		demonstrates := slices.Contains(card.Demonstrated, slug)
		if !assesses && !demonstrates {
			continue
		}
		demonstratedOnly := !assesses && demonstrates

		cardCounts := false
		for _, a := range card.Attempts {
			tier, graded := attemptTier(a, slug)
			if !graded {
				continue // this sitting didn't grade this particular skill
			}
			if demonstratedOnly && tier != tierFail && tier != stateGreen {
				continue // invisible to the ladder -- not a climb, not a slip, not evidence
			}
			events = append(events, ladderEvent{
				date:         a.Date,
				tier:         tier,
				demonstrated: demonstratedOnly,
			})
			cardCounts = true
		}
		if cardCounts {
			evidenceTests = append(evidenceTests, card.Name)
		}
	}
	if len(events) == 0 {
		return stateNone, false, nil
	}

	sort.SliceStable(events, func(i, j int) bool { return events[i].date < events[j].date })

	state = stateNone
	greenStreak := 0
	for _, e := range events {
		if e.tier == tierFail {
			state = stepDown(state)
			greenStreak = 0
			continue
		}
		if rungOrder[e.tier] > rungOrder[state] {
			state = e.tier
		}
		if e.tier == stateGreen && state == stateGreen {
			greenStreak++
			if greenStreak >= greensForMaster {
				state = stateMaster
			}
		}
	}
	return state, true, evidenceTests
}
