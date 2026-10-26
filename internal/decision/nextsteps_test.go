package decision

import (
	"strings"
	"testing"

	"matritsa/internal/scoring"
)

func TestNextSteps(t *testing.T) {
	cases := []struct {
		name    string
		reasons []string
		reapply bool
		want    string
	}{
		{"liquidated company", []string{scoring.StopInactive}, false, "прекратила деятельность"},
		{"class C without collateral", []string{ReasonNeedsCollateral}, true, "Добавьте обеспечение"},
		{"company unknown to the registry", []string{scoring.StopNotFound}, true, "ИНН"},
		{"bankruptcy", []string{scoring.StopBankrupt}, false, "банкротства"},
		{"disqualified director", []string{scoring.StopDisqualified}, false, "смены руководителя"},
		{"unreliable registry data", []string{scoring.StopUnreliable}, false, "Обновите сведения"},
		{"overdue with the number of days", []string{scoring.StopOverduePrefix + " 90 дней"}, false, "Погасите"},
		{"no revenue", []string{scoring.StopNoRevenue}, true, "выписку"},
		{"low score", []string{"Низкая оценка надёжности"}, true, "меньшую сумму"},
		{"no reasons", nil, true, "меньшую сумму"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			advice, reapply := NextSteps(c.reasons)
			if reapply != c.reapply || !strings.Contains(advice, c.want) {
				t.Fatalf("NextSteps(%q) = %q, %v; want %q, %v", c.reasons, advice, reapply, c.want, c.reapply)
			}
		})
	}
}

func TestHardStopNeverSuggestsSmallerAmount(t *testing.T) {
	for _, stop := range []string{scoring.StopInactive, scoring.StopBankrupt, scoring.StopDisqualified, scoring.StopUnreliable, scoring.StopOverduePrefix + " 90 дней"} {
		if advice, _ := NextSteps([]string{stop}); strings.Contains(advice, "меньшую сумму") {
			t.Errorf("%q: advice suggests a smaller amount: %q", stop, advice)
		}
	}
}
