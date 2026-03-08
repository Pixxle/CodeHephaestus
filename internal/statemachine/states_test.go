package statemachine

import "testing"

func TestStateConstantsDistinct(t *testing.T) {
	states := []State{
		StateTodo,
		StatePlanning,
		StatePlanningReady,
		StateCIFailure,
		StateInReview,
		StateDone,
	}

	seen := make(map[State]bool)
	for _, s := range states {
		if seen[s] {
			t.Errorf("duplicate state constant: %q", s)
		}
		seen[s] = true
	}
}

func TestValidTransitionsNoDuplicates(t *testing.T) {
	type pair struct {
		from, to State
	}
	seen := make(map[pair]bool)

	for _, tr := range ValidTransitions {
		p := pair{tr.From, tr.To}
		if seen[p] {
			t.Errorf("duplicate transition: %s -> %s", tr.From, tr.To)
		}
		seen[p] = true
	}
}

func TestValidTransitionsHaveTriggers(t *testing.T) {
	for _, tr := range ValidTransitions {
		if tr.Trigger == "" {
			t.Errorf("transition %s -> %s has empty trigger", tr.From, tr.To)
		}
	}
}
