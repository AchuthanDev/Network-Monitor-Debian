package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/apps/backend/internal/db"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

type dashboardResponse struct {
	Status      string         `json:"status"`
	Message     string         `json:"message,omitempty"`
	Today       db.UsageTotals `json:"today"`
	GeneratedAt string         `json:"generated_at"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(0)
	}

	bind := getenv("NETWORK_MONITOR_BIND_ADDRESS", "0.0.0.0")
	port := getenv("NETWORK_MONITOR_API_PORT", "8080")
	databaseURL := getenv("NETWORK_MONITOR_DATABASE_URL", "")

	ctx := context.Background()
	store, err := db.New(ctx, databaseURL)
	if err != nil {
		slog.Warn("database unavailable", "error", err)
	} else {
		defer store.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", writeHealth)
	mux.HandleFunc("GET /api/v1/health", writeHealth)
	mux.HandleFunc("GET /api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
		writeDashboard(w, r, store)
	})
	mux.HandleFunc("GET /api/v1/network/hourly", func(w http.ResponseWriter, r *http.Request) {
		writeHourly(w, r, store)
	})

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

func writeDashboard(w http.ResponseWriter, r *http.Request, store *db.Store) {
	if store == nil {
		writeJSON(w, http.StatusOK, dashboardResponse{
			Status:      "unavailable",
			Message:     "Database is unavailable. No production metrics are fabricated.",
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	hasData, err := store.HasTrafficData(r.Context())
	if err != nil || !hasData {
		writeJSON(w, http.StatusOK, dashboardResponse{
			Status:      "unavailable",
			Message:     "No verified traffic samples are available yet.",
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	totals, err := store.TodayTotals(r.Context(), time.Now())
	if err != nil {
		writeJSON(w, http.StatusOK, dashboardResponse{
			Status:      "unavailable",
			Message:     "Traffic totals are unavailable from the database.",
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	writeJSON(w, http.StatusOK, dashboardResponse{
		Status:      "ok",
		Today:       totals,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func writeHourly(w http.ResponseWriter, r *http.Request, store *db.Store) {
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":  "unavailable",
			"message": "database unavailable",
		})
		return
	}
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	buckets, err := store.Hourly(r.Context(), from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"status":  "error",
			"message": "failed to query hourly traffic",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"from":   from.Format(time.RFC3339),
		"to":     to.Format(time.RFC3339),
		"data":   buckets,
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
