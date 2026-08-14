package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
)

type healthResponse struct {
	Status      string   `json:"status"`
	Service     string   `json:"service"`
	Mode        string   `json:"mode"`
	Unavailable []string `json:"unavailable"`
	Timestamp   string   `json:"timestamp"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(0)
	}

	bind := getenv("NETWORK_MONITOR_BIND_ADDRESS", "0.0.0.0")
	port := getenv("NETWORK_MONITOR_COLLECTOR_PORT", "9091")
	_ = classifier.DefaultConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, healthResponse{
			Status:  "ok",
			Service: "network-monitor-collector",
			Mode:    "bootstrap",
			Unavailable: []string{
				"ebpf_collection",
				"process_attribution",
				"container_attribution",
				"database_writer",
			},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	})

	server := &http.Server{
		Addr:              bind + ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("network monitor collector listening", "addr", server.Addr, "mode", "bootstrap")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("collector stopped", "error", err)
		os.Exit(1)
	}
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
