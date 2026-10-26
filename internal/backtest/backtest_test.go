package backtest

import (
	"testing"

	"matritsa/internal/scoring"
)

func TestAUC_KnownCases(t *testing.T) {
	type s struct {
		score int
		bad   bool
	}
	get := func(x s) (int, bool) { return x.score, x.bad }
	if got := auc([]s{{10, true}, {20, false}}, get); got != 1 {
		t.Errorf("perfect ranking: %v", got)
	}
	if got := auc([]s{{20, true}, {10, false}}, get); got != 0 {
		t.Errorf("reversed ranking: %v", got)
	}
	if got := auc([]s{{10, true}, {10, false}}, get); got != 0.5 {
		t.Errorf("a tie counts half: %v", got)
	}
}

// The scorecard must rank the synthetic risk well and hold more defaults in each worse class.
// The thresholds are deliberately modest: the "true" world is an assumption (see TrueLogit).
func TestScorecardSeparatesSyntheticBorrowers(t *testing.T) {
	rep := Run(6000, 42)
	// A coarse points card on a world with hidden risk: about a quarter is what it reaches (see docs/scoring.md).
	// The check guards against a change that breaks the ranking, not for a target value.
	if rep.Gini < 0.15 || rep.Gini > rep.OracleGini {
		t.Errorf("Gini %.2f (oracle %.2f): the scorecard does not rank the risk", rep.Gini, rep.OracleGini)
	}
	if !rep.Monotonic {
		t.Errorf("the share of defaults must grow from A to D: %+v", rep.Grades)
	}
	for _, g := range rep.Grades {
		if g.Grade == scoring.GradeA && g.Rate > 0.08 {
			t.Errorf("class A holds too many defaults: %.1f %%", g.Rate*100)
		}
	}
}
