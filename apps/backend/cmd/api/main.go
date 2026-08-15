package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/apps/backend/internal/db"
	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/discovery"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/isp"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/plan"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/readiness"
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

type devicesResponse struct {
	Status  string              `json:"status"`
	Mode    gatewayconfig.Mode  `json:"mode"`
	Message string              `json:"message"`
	Data    []map[string]string `json:"data"`
}

type ispUsageResponse struct {
	Status       string             `json:"status"`
	Mode         gatewayconfig.Mode `json:"mode"`
	Window       isp.WindowConfig   `json:"window"`
	Scope        string             `json:"scope"`
	FreeBytes    uint64             `json:"free_night_bytes"`
	AnytimeBytes uint64             `json:"anytime_bytes"`
	TotalBytes   uint64             `json:"total_bytes"`
	Hourly       []ispHourlyRow     `json:"hourly"`
	GeneratedAt  string             `json:"generated_at"`
	Message      string             `json:"message,omitempty"`
}

type ispHourlyRow struct {
	BucketStart string     `json:"bucket_start"`
	Period      isp.Period `json:"period"`
	Bytes       uint64     `json:"bytes"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(0)
	}

	bind := getenv("NETWORK_MONITOR_BIND_ADDRESS", "0.0.0.0")
	port := getenv("NETWORK_MONITOR_API_PORT", "8080")
	databaseURL := getenv("NETWORK_MONITOR_DATABASE_URL", "")
	gatewayCfg := gatewayconfig.LoadFromEnv()

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
	mux.HandleFunc("GET /api/v1/gateway/discovery", func(w http.ResponseWriter, r *http.Request) {
		writeGatewayDiscovery(w, r, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/gateway/readiness", func(w http.ResponseWriter, r *http.Request) {
		writeGatewayReadiness(w, r, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/gateway/plan", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, plan.BuildDryRun(gatewayCfg))
	})
	mux.HandleFunc("GET /api/v1/devices", func(w http.ResponseWriter, _ *http.Request) {
		writeDevices(w, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/isp-usage", func(w http.ResponseWriter, r *http.Request) {
		writeISPUsage(w, r, store, gatewayCfg)
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

func writeGatewayDiscovery(w http.ResponseWriter, r *http.Request, cfg gatewayconfig.Config) {
	report := discovery.Discover(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"mode":      cfg.Mode,
		"config":    cfg,
		"discovery": report,
	})
}

func writeGatewayReadiness(w http.ResponseWriter, r *http.Request, cfg gatewayconfig.Config) {
	report := discovery.Discover(r.Context())
	writeJSON(w, http.StatusOK, readiness.Evaluate(cfg, report))
}

func writeDevices(w http.ResponseWriter, cfg gatewayconfig.Config) {
	message := "Gateway mode is not enabled yet. No monitored LAN clients are being collected."
	if cfg.Mode == gatewayconfig.ModeGateway {
		message = "Gateway mode is configured, but live per-device collection is not enabled in this phase."
	}
	writeJSON(w, http.StatusOK, devicesResponse{
		Status:  "ok",
		Mode:    cfg.Mode,
		Message: message,
		Data:    []map[string]string{},
	})
}

func writeISPUsage(w http.ResponseWriter, r *http.Request, store *db.Store, cfg gatewayconfig.Config) {
	window := cfg.ISPWindow()
	response := ispUsageResponse{
		Status:      "unavailable",
		Mode:        cfg.Mode,
		Window:      window,
		Scope:       "server",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if store == nil {
		response.Message = "database unavailable"
		writeJSON(w, http.StatusOK, response)
		return
	}

	to := time.Now().UTC()
	from := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	buckets, err := store.Hourly(r.Context(), from, to.Add(time.Hour))
	if err != nil {
		response.Message = "failed to query hourly traffic"
		writeJSON(w, http.StatusOK, response)
		return
	}

	response.Status = "ok"
	for _, bucket := range buckets {
		if bucket.TrafficClass != "internet" {
			continue
		}
		bytes := bucket.DownloadBytes + bucket.UploadBytes
		period, err := window.PeriodAt(bucket.BucketStart)
		if err != nil {
			response.Status = "unavailable"
			response.Message = err.Error()
			writeJSON(w, http.StatusOK, response)
			return
		}
		if period == isp.PeriodFreeNight {
			response.FreeBytes += bytes
		} else {
			response.AnytimeBytes += bytes
		}
		response.Hourly = append(response.Hourly, ispHourlyRow{
			BucketStart: bucket.BucketStart.Format(time.RFC3339),
			Period:      period,
			Bytes:       bytes,
		})
	}
	response.TotalBytes = response.FreeBytes + response.AnytimeBytes
	writeJSON(w, http.StatusOK, response)
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
