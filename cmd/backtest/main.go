// Command backtest scores a synthetic portfolio and prints how well the scorecard separates the
// borrowers who default (Gini, KS) and the share of defaults in each class against the configured PD.
//
//	go run ./cmd/backtest            # 20 000 companies, fixed seed
//	go run ./cmd/backtest -n 50000 -seed 7
//
// The "true" chance of default is an assumption (internal/backtest.TrueLogit), so the result shows that
// the scorecard ranks risk sensibly, not that its parameters are calibrated.
package main

import (
	"flag"
	"fmt"

	"matritsa/internal/backtest"
)

func main() {
	n := flag.Int("n", 20000, "number of synthetic companies")
	seed := flag.Uint64("seed", 2026, "random seed")
	flag.Parse()
	rep := backtest.Run(*n, *seed)
	fmt.Printf("Компаний: %d, дефолтов за срок: %d (%.1f %%)\n", rep.N, rep.Defaults, 100*float64(rep.Defaults)/float64(rep.N))
	fmt.Printf("Gini: %.2f   KS: %.2f   Доля дефолтов растёт от A к D: %v\n", rep.Gini, rep.KS, map[bool]string{true: "да", false: "нет"}[rep.Monotonic])
	fmt.Printf("Потолок (ранжирование по истинной PD): Gini %.2f, модель берёт %.0f %% достижимого\n\n", rep.OracleGini, 100*rep.Gini/rep.OracleGini)
	fmt.Println("| Класс | Заявок | Доля | Дефолтов | Наблюдаемая доля дефолтов | PD модели за средний срок |")
	fmt.Println("|---|---:|---:|---:|---:|---:|")
	for _, g := range rep.Grades {
		fmt.Printf("| %s | %d | %.0f %% | %d | %.1f %% | %.1f %% (%.0f мес.) |\n", g.Grade, g.Count, 100*float64(g.Count)/float64(rep.N), g.Defaults, 100*g.Rate, 100*g.ModelPD, g.AverageTerm)
	}
}
