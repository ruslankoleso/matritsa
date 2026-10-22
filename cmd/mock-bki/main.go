// Command mock-bki is the simulated credit bureau service.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"matritsa/internal/mocks"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	ms, _ := strconv.Atoi(os.Getenv("MOCK_LATENCY_MS"))
	h := mocks.BKI(mocks.Options{Latency: time.Duration(ms) * time.Millisecond, Fail: new(mocks.FailSwitch)})
	slog.Info("mock-bki listening", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
