package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/apps/backend/internal/db"
	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/discovery"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/isp"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/plan"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/readiness"
	classdomain "github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/domain"
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

type listResponse[T any] struct {
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	Data        []T    `json:"data"`
	GeneratedAt string `json:"generated_at"`
}

type reportResponse struct {
	Status      string                    `json:"status"`
	Message     string                    `json:"message,omitempty"`
	Report      *db.DeviceReport          `json:"report,omitempty"`
	Categories  []db.CategoryBreakdownRow `json:"categories,omitempty"`
	GeneratedAt string                    `json:"generated_at"`
}

type dailyReportResponse struct {
	Status                 string  `json:"status"`
	Message                string  `json:"message,omitempty"`
	Date                   string  `json:"date"`
	Internet               uint64  `json:"internet_bytes"`
	Free                   uint64  `json:"free_night_bytes"`
	Anytime                uint64  `json:"anytime_bytes"`
	ClassifiedBytes        uint64  `json:"classified_bytes"`
	UnknownBytes           uint64  `json:"unknown_bytes"`
	ClassificationCoverage float64 `json:"classification_coverage"`
	TopService             string  `json:"top_service,omitempty"`
	TopCategory            string  `json:"top_category,omitempty"`
	PeakHour               string  `json:"peak_hour,omitempty"`
	Alerts                 uint64  `json:"alerts"`
	GeneratedAt            string  `json:"generated_at"`
}

type wizardResponse struct {
	Status      string               `json:"status"`
	ApplyReady  bool                 `json:"apply_ready"`
	Steps       []wizardStep         `json:"steps"`
	Warnings    []string             `json:"warnings"`
	Config      gatewayconfig.Config `json:"config"`
	Readiness   readiness.Report     `json:"readiness"`
	GeneratedAt string               `json:"generated_at"`
}

type wizardStep struct {
	Step     int    `json:"step"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Detail   string `json:"detail"`
	Disabled bool   `json:"disabled,omitempty"`
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
	mux.HandleFunc("GET /api/v1/gateway/wizard", func(w http.ResponseWriter, r *http.Request) {
		writeGatewayWizard(w, r, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/devices", func(w http.ResponseWriter, _ *http.Request) {
		writeDevices(w, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/isp-usage", func(w http.ResponseWriter, r *http.Request) {
		writeISPUsage(w, r, store, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/destinations", func(w http.ResponseWriter, r *http.Request) {
		writeDestinations(w, r, store)
	})
	mux.HandleFunc("GET /api/v1/reports/device", func(w http.ResponseWriter, r *http.Request) {
		writeDeviceReport(w, r, store)
	})
	mux.HandleFunc("GET /api/v1/investigation/hour", func(w http.ResponseWriter, r *http.Request) {
		writeHourlyInvestigation(w, r, store)
	})
	mux.HandleFunc("GET /api/v1/reports/daily", func(w http.ResponseWriter, r *http.Request) {
		writeDailyReport(w, r, store, gatewayCfg)
	})
	mux.HandleFunc("GET /api/v1/classification/catalog", writeClassificationCatalog)
	mux.HandleFunc("GET /api/v1/alerts/policy", writeAlertPolicy)

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

func writeGatewayWizard(w http.ResponseWriter, r *http.Request, cfg gatewayconfig.Config) {
	discovered := discovery.Discover(r.Context())
	ready := readiness.Evaluate(cfg, discovered)
	lanSelected := cfg.Gateway.LANInterface != ""
	wan := cfg.Gateway.WANInterface
	if wan == "" {
		wan = discovered.WANInterface
	}
	response := wizardResponse{
		Status:      "ok",
		ApplyReady:  ready.Ready && lanSelected,
		Config:      cfg,
		Readiness:   ready,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Steps: []wizardStep{
			{Step: 1, Name: "WAN interface", Status: statusFor(wan != ""), Detail: valueOr(wan, "Auto-detect unavailable")},
			{Step: 2, Name: "Dedicated LAN interface", Status: statusFor(lanSelected), Detail: valueOr(cfg.Gateway.LANInterface, "No suitable dedicated interface detected"), Disabled: !lanSelected},
			{Step: 3, Name: "LAN addressing", Status: "ready", Detail: cfg.Gateway.GatewayIP + " / " + cfg.Gateway.LANCIDR},
			{Step: 4, Name: "DHCP", Status: statusFor(cfg.Gateway.DHCP.Enabled), Detail: cfg.Gateway.DHCP.RangeStart + "-" + cfg.Gateway.DHCP.RangeEnd},
			{Step: 5, Name: "DNS/Pi-hole", Status: string(cfg.Gateway.DNS.Mode), Detail: "Pi-hole remains the DNS evidence source; no DNS changes are applied here"},
			{Step: 6, Name: "nftables/NAT/accounting review", Status: "dry_run", Detail: "Generated plan only; no live rules are installed from the dashboard"},
			{Step: 7, Name: "Connectivity safety check", Status: statusFor(checkStatus(ready, "ssh_management_preserved") == "pass"), Detail: "SSH management must remain on the WAN-side 192.168.1.x path"},
			{Step: 8, Name: "Rollback safety", Status: statusFor(checkStatus(ready, "rollback_plan_available") == "pass" && checkStatus(ready, "automatic_rollback_ready") == "pass"), Detail: "Future live apply requires a rollback plan and 120-second confirmation timer"},
			{Step: 9, Name: "Apply", Status: statusFor(ready.Ready && lanSelected), Detail: "Disabled until dedicated LAN interface is selected and all required checks pass", Disabled: !(ready.Ready && lanSelected)},
		},
	}
	if !lanSelected {
		response.Warnings = append(response.Warnings, "Dedicated LAN interface is not connected or selected")
	}
	writeJSON(w, http.StatusOK, response)
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

func writeDestinations(w http.ResponseWriter, r *http.Request, store *db.Store) {
	response := listResponse[db.DestinationRow]{
		Status:      "unavailable",
		Message:     "destination analytics are unavailable until gateway/device accounting writes verified rows",
		Data:        []db.DestinationRow{},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if store == nil {
		response.Message = "database unavailable"
		writeJSON(w, http.StatusOK, response)
		return
	}
	from, to := parseRange(r, 24*time.Hour)
	rows, err := store.Destinations(r.Context(), from, to)
	if err != nil {
		response.Message = "destination query unavailable: " + err.Error()
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Status = "ok"
	response.Message = ""
	response.Data = rows
	writeJSON(w, http.StatusOK, response)
}

func writeDeviceReport(w http.ResponseWriter, r *http.Request, store *db.Store) {
	response := reportResponse{Status: "unavailable", Message: "device reports require verified gateway device accounting rows", GeneratedAt: time.Now().UTC().Format(time.RFC3339)}
	if store == nil {
		response.Message = "database unavailable"
		writeJSON(w, http.StatusOK, response)
		return
	}
	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		response.Message = "device_id is required"
		writeJSON(w, http.StatusBadRequest, response)
		return
	}
	from, to := parseRange(r, 24*time.Hour)
	report, categories, err := store.DeviceReport(r.Context(), deviceID, from, to)
	if err != nil {
		response.Message = "device report unavailable: " + err.Error()
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Status = "ok"
	response.Message = ""
	response.Report = &report
	response.Categories = categories
	writeJSON(w, http.StatusOK, response)
}

func writeHourlyInvestigation(w http.ResponseWriter, r *http.Request, store *db.Store) {
	query := r.URL.Query()
	dateValue := query.Get("date")
	hourValue := query.Get("hour")
	if hourValue == "" {
		hourValue = "0"
	}
	hour, err := strconv.Atoi(hourValue)
	if err != nil || hour < 0 || hour > 23 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "unavailable", "message": "hour must be 0-23"})
		return
	}
	if dateValue == "" {
		dateValue = time.Now().UTC().Format("2006-01-02")
	}
	day, err := time.Parse("2006-01-02", dateValue)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"status": "unavailable", "message": "date must be YYYY-MM-DD"})
		return
	}
	start := day.Add(time.Duration(hour) * time.Hour)
	request := r.Clone(r.Context())
	values := request.URL.Query()
	values.Set("from", start.Format(time.RFC3339))
	values.Set("to", start.Add(time.Hour).Format(time.RFC3339))
	request.URL.RawQuery = values.Encode()
	writeDeviceReport(w, request, store)
}

func writeDailyReport(w http.ResponseWriter, r *http.Request, store *db.Store, cfg gatewayconfig.Config) {
	dateValue := r.URL.Query().Get("date")
	if dateValue == "" {
		dateValue = time.Now().UTC().Format("2006-01-02")
	}
	response := dailyReportResponse{
		Status:      "unavailable",
		Message:     "daily report uses server metrics until gateway device accounting is active",
		Date:        dateValue,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if store == nil {
		response.Message = "database unavailable"
		writeJSON(w, http.StatusOK, response)
		return
	}
	day, err := time.Parse("2006-01-02", dateValue)
	if err != nil {
		response.Message = "date must be YYYY-MM-DD"
		writeJSON(w, http.StatusBadRequest, response)
		return
	}
	buckets, err := store.Hourly(r.Context(), day, day.Add(24*time.Hour))
	if err != nil {
		response.Message = "daily report unavailable: " + err.Error()
		writeJSON(w, http.StatusOK, response)
		return
	}
	window := cfg.ISPWindow()
	response.Status = "ok"
	response.Message = ""
	for _, bucket := range buckets {
		if bucket.TrafficClass != "internet" {
			continue
		}
		bytes := bucket.DownloadBytes + bucket.UploadBytes
		response.Internet += bytes
		period, err := window.PeriodAt(bucket.BucketStart)
		if err == nil && period == isp.PeriodFreeNight {
			response.Free += bytes
		} else {
			response.Anytime += bytes
		}
	}
	if summary, err := store.ClassificationSummary(r.Context(), day, day.Add(24*time.Hour)); err == nil {
		response.ClassifiedBytes = summary.ClassifiedBytes
		response.UnknownBytes = summary.UnknownBytes
		response.TopService = summary.TopService
		response.TopCategory = summary.TopCategory
		if summary.ClassifiedBytes+summary.UnknownBytes > 0 {
			response.ClassificationCoverage = float64(summary.ClassifiedBytes) / float64(summary.ClassifiedBytes+summary.UnknownBytes)
		}
	}
	writeJSON(w, http.StatusOK, response)
}

func writeClassificationCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"privacy": []string{
			"no_tls_interception",
			"no_client_certificates",
			"no_payload_storage",
			"metadata_and_dns_correlation_only",
		},
		"confidence": []classdomain.Confidence{classdomain.ConfidenceHigh, classdomain.ConfidenceMedium, classdomain.ConfidenceLow, classdomain.ConfidenceUnknown},
		"categories": []classdomain.Category{
			classdomain.CategoryVideoStreaming,
			classdomain.CategorySocialMedia,
			classdomain.CategoryDownloads,
			classdomain.CategorySoftwareUpdate,
			classdomain.CategoryGeneralWeb,
			classdomain.CategoryDNS,
			classdomain.CategoryQUIC,
			classdomain.CategoryUnknownHTTPS,
			classdomain.CategoryOther,
		},
	})
}

func writeAlertPolicy(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"defaults": []map[string]any{
			{"name": "Anytime usage > 2 GB/device/day", "threshold_bytes": 2_000_000_000, "scope": "device_anytime"},
			{"name": "Social/video > 2 GB/device/day", "threshold_bytes": 2_000_000_000, "categories": []string{"social_media", "video_streaming"}},
			{"name": "Unknown traffic > 2 GB/device/day", "threshold_bytes": 2_000_000_000, "categories": []string{"unknown_https"}},
			{"name": "1 GB transferred within 10 minutes", "threshold_bytes": 1_000_000_000, "window": "10m"},
			{"name": "Unusual upload spike", "threshold_bytes": 500_000_000, "direction": "upload"},
			{"name": "New device detected", "scope": "device_identity"},
			{"name": "Inactive device significant usage", "threshold_bytes": 500_000_000, "scope": "device_reactivation"},
		},
		"dedupe": map[string]any{"tiers_bytes": []uint64{2_000_000_000, 5_000_000_000, 10_000_000_000}, "cooldown": "24h"},
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

func parseRange(r *http.Request, fallback time.Duration) (time.Time, time.Time) {
	now := time.Now().UTC()
	from := now.Add(-fallback)
	to := now
	if value := r.URL.Query().Get("from"); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			from = parsed
		}
	}
	if value := r.URL.Query().Get("to"); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			to = parsed
		}
	}
	return from, to
}

func statusFor(ok bool) string {
	if ok {
		return "ready"
	}
	return "blocked"
}

func checkStatus(report readiness.Report, name string) string {
	for _, check := range report.Checks {
		if check.Name == name {
			return string(check.Status)
		}
	}
	return ""
}

func valueOr(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
