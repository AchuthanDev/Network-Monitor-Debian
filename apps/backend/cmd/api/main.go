package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

type dashboardResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(0)
	}

	bind := getenv("NETWORK_MONITOR_BIND_ADDRESS", "0.0.0.0")
	port := getenv("NETWORK_MONITOR_API_PORT", "8080")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", writeHealth)
	mux.HandleFunc("GET /api/v1/health", writeHealth)
	mux.HandleFunc("GET /api/v1/dashboard", writeDashboard)

	server := &http.Server{
		Addr:              bind + ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("network monitor api listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func writeHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Service:   "network-monitor-api",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func writeDashboard(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, dashboardResponse{
		Status:  "unavailable",
		Message: "Real traffic accounting starts in Phase 2. No production metrics are fabricated.",
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write response", "error", err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
