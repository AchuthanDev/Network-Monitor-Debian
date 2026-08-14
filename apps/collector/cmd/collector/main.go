package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/apps/collector/internal/config"
	"github.com/AchuthanDev/Network-Monitor-Debian/apps/collector/internal/db"
	"github.com/AchuthanDev/Network-Monitor-Debian/apps/collector/internal/host"
	"github.com/AchuthanDev/Network-Monitor-Debian/apps/collector/internal/nft"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/accounting"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/conntrack"
)

type healthResponse struct {
	Status       string         `json:"status"`
	Service      string         `json:"service"`
	Mode         string         `json:"mode"`
	Accounting   string         `json:"accounting"`
	LastSampleAt string         `json:"last_sample_at,omitempty"`
	LastError    string         `json:"last_error,omitempty"`
	Totals       sampleTotals   `json:"totals"`
	Route        host.RouteInfo `json:"route"`
	Timestamp    string         `json:"timestamp"`
}

type sampleTotals struct {
	SamplesRead      uint64 `json:"samples_read"`
	DeltasWritten    uint64 `json:"deltas_written"`
	InternetDownload uint64 `json:"internet_download_bytes"`
	InternetUpload   uint64 `json:"internet_upload_bytes"`
	LANDownload      uint64 `json:"lan_download_bytes"`
	LANUpload        uint64 `json:"lan_upload_bytes"`
	DockerDownload   uint64 `json:"docker_download_bytes"`
	DockerUpload     uint64 `json:"docker_upload_bytes"`
}

type collectorState struct {
	mu           sync.RWMutex
	status       string
	mode         string
	accounting   string
	lastSampleAt time.Time
	lastError    string
	route        host.RouteInfo
	totals       sampleTotals
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(0)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	state := &collectorState{status: "starting", mode: "starting", accounting: "unavailable"}

	route, err := host.Detect(cfg.HostProcRoot)
	if err != nil {
		state.setError("detect host network", err)
	} else {
		state.setRoute(route)
	}

	writer, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		state.setError("connect database", err)
	} else {
		defer writer.Close()
		go runCollector(ctx, cfg, writer, state, route)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, state.health())
	})

	server := &http.Server{
		Addr:              cfg.BindAddress + ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("network monitor collector listening", "addr", server.Addr, "mode", "conntrack_snapshot")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("collector stopped", "error", err)
		os.Exit(1)
	}
}

func runCollector(ctx context.Context, cfg config.Config, writer *db.Writer, state *collectorState, route host.RouteInfo) {
	if err := nft.Setup(ctx, route.DefaultInterface); err != nil {
		state.setError("setup nftables accounting", err)
		runConntrackCollector(ctx, cfg, writer, state, route)
		return
	}
	state.mu.Lock()
	state.mode = "nftables_counters"
	state.accounting = "nftables_counters"
	state.status = "starting"
	state.mu.Unlock()

	var previous nft.CounterSnapshot
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := nft.ReadCounters(ctx)
			if err != nil {
				state.setError("read nftables counters", err)
				continue
			}
			deltas := current.Deltas(previous).ToTrafficDeltas(time.Now().UTC())
			previous = current
			if err := writer.WriteDeltas(ctx, deltas); err != nil {
				state.setError("write nftables deltas", err)
				continue
			}
			state.record("nftables_counters", deltas)
		}
	}
}

func runConntrackCollector(ctx context.Context, cfg config.Config, writer *db.Writer, state *collectorState, route host.RouteInfo) {
	conntrackPath := filepath.Join(cfg.HostProcRoot, "net", "nf_conntrack")
	acctPath := filepath.Join(cfg.HostProcRoot, "sys", "net", "netfilter", "nf_conntrack_acct")

	if enabled, err := conntrackAccountingEnabled(acctPath); err != nil {
		state.setError("check conntrack accounting", err)
		return
	} else if !enabled {
		state.setError("check conntrack accounting", errors.New("nf_conntrack_acct is disabled; byte-accurate conntrack accounting unavailable"))
		return
	}
	state.mu.Lock()
	state.mode = "conntrack_snapshot"
	state.accounting = "conntrack_snapshot"
	state.status = "starting"
	state.mu.Unlock()

	matcher := accounting.NewLocalMatcher(host.HostIPs(route), append(host.LocalCIDRs(route), cfg.Classifier.DockerCIDRs...))
	previous := map[string]conntrack.Counters{}
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deltas, next, err := sampleConntrack(conntrackPath, previous, matcher, cfg.Classifier)
			if err != nil {
				state.setError("sample conntrack", err)
				continue
			}
			previous = next

			if err := writer.WriteDeltas(ctx, deltas); err != nil {
				state.setError("write traffic deltas", err)
				continue
			}

			state.record("conntrack_snapshot", deltas)
		}
	}
}

func sampleConntrack(path string, previous map[string]conntrack.Counters, matcher accounting.LocalMatcher, cfg classifier.Config) ([]accounting.TrafficDelta, map[string]conntrack.Counters, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, previous, err
	}
	defer file.Close()

	flows, err := conntrack.ParseReader(file)
	if err != nil {
		return nil, previous, err
	}

	now := time.Now().UTC()
	next := make(map[string]conntrack.Counters, len(flows))
	deltas := make([]accounting.TrafficDelta, 0, len(flows))
	for _, flow := range flows {
		key := flow.Key()
		counter := flow.Counters
		next[key] = counter
		if prev, ok := previous[key]; ok {
			if delta, ok := accounting.BuildDelta(now, flow, &prev, matcher, cfg); ok {
				deltas = append(deltas, delta)
			}
		}
	}
	return deltas, next, nil
}

func conntrackAccountingEnabled(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return string(raw[:1]) == "1", nil
}

func (s *collectorState) setRoute(route host.RouteInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.route = route
}

func (s *collectorState) setError(operation string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = "degraded"
	s.accounting = "unavailable"
	s.lastError = fmt.Sprintf("%s: %v", operation, err)
	slog.Warn("collector degraded", "operation", operation, "error", err)
}

func (s *collectorState) record(mode string, deltas []accounting.TrafficDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = "ok"
	s.mode = mode
	s.accounting = mode
	s.lastError = ""
	s.lastSampleAt = time.Now().UTC()
	s.totals.SamplesRead++
	s.totals.DeltasWritten += uint64(len(deltas))
	for _, delta := range deltas {
		switch delta.Class {
		case classifier.TrafficInternet:
			s.totals.InternetDownload += delta.DownloadBytes
			s.totals.InternetUpload += delta.UploadBytes
		case classifier.TrafficLAN:
			s.totals.LANDownload += delta.DownloadBytes
			s.totals.LANUpload += delta.UploadBytes
		case classifier.TrafficDocker:
			s.totals.DockerDownload += delta.DownloadBytes
			s.totals.DockerUpload += delta.UploadBytes
		}
	}
}

func (s *collectorState) health() healthResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lastSample := ""
	if !s.lastSampleAt.IsZero() {
		lastSample = s.lastSampleAt.Format(time.RFC3339)
	}
	status := s.status
	if status == "" {
		status = "starting"
	}
	return healthResponse{
		Status:       status,
		Service:      "network-monitor-collector",
		Mode:         s.mode,
		Accounting:   s.accounting,
		LastSampleAt: lastSample,
		LastError:    s.lastError,
		Totals:       s.totals,
		Route:        s.route,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write response", "error", err)
	}
}
