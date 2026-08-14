import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { CheckCircle2, Database, RefreshCw, Server, Wifi } from "lucide-react";
import { formatBytes, formatDateTime } from "./lib/format";
import "./styles.css";

type UsageTotals = {
  internet_download_bytes: number;
  internet_upload_bytes: number;
  lan_download_bytes: number;
  lan_upload_bytes: number;
  docker_download_bytes: number;
  docker_upload_bytes: number;
};

type DashboardResponse = {
  status: string;
  message?: string;
  today: UsageTotals;
  generated_at: string;
};

type CollectorResponse = {
  status: string;
  mode: string;
  accounting: string;
  last_sample_at?: string;
  last_error?: string;
  totals: UsageTotals & {
    samples_read: number;
    deltas_written: number;
  };
  route: {
    default_interface: string;
    default_gateway: string;
  };
};

type HourlyBucket = {
  bucket_start: string;
  traffic_class: string;
  download_bytes: number;
  upload_bytes: number;
};

type HourlyResponse = {
  status: string;
  data: HourlyBucket[];
};

function MetricCard({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <section className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </section>
  );
}

function StatusPill({ collector }: { collector: CollectorResponse | null }) {
  const ok = collector?.status === "ok";
  return (
    <div className={`status-pill ${ok ? "ok" : "warning"}`}>
      <CheckCircle2 size={16} />
      {collector ? `${collector.accounting} ${ok ? "active" : "degraded"}` : "Loading"}
    </div>
  );
}

function App() {
  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [collector, setCollector] = useState<CollectorResponse | null>(null);
  const [hourly, setHourly] = useState<HourlyBucket[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  async function load() {
    try {
      const [dashboardResponse, collectorResponse, hourlyResponse] = await Promise.all([
        fetch("/api/v1/dashboard"),
        fetch("/collector/healthz"),
        fetch("/api/v1/network/hourly"),
      ]);
      if (!dashboardResponse.ok || !collectorResponse.ok || !hourlyResponse.ok) {
        throw new Error("A monitoring endpoint failed");
      }
      const dashboardData = (await dashboardResponse.json()) as DashboardResponse;
      const collectorData = (await collectorResponse.json()) as CollectorResponse;
      const hourlyData = (await hourlyResponse.json()) as HourlyResponse;
      setDashboard(dashboardData);
      setCollector(collectorData);
      setHourly(hourlyData.data ?? []);
      setUpdatedAt(new Date());
      setError(null);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "Failed to load monitoring data");
    }
  }

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 5000);
    return () => window.clearInterval(timer);
  }, []);

  const today = dashboard?.today;
  const totalInternet = (today?.internet_download_bytes ?? 0) + (today?.internet_upload_bytes ?? 0);
  const totalLAN = (today?.lan_download_bytes ?? 0) + (today?.lan_upload_bytes ?? 0);
  const recentHourly = useMemo(() => hourly.slice(-8).reverse(), [hourly]);

  return (
    <main className="shell">
      <aside className="sidebar">
        <div className="brand">
          <Server size={22} />
          <span>Network Monitor Debian</span>
        </div>
        <nav>
          <a className="active" href="/">Overview</a>
          <a href="/">Network</a>
          <a href="/">Containers</a>
          <a href="/">Processes</a>
          <a href="/">Connections</a>
          <a href="/">Server</a>
          <a href="/">Alerts</a>
          <a href="/">Reports</a>
          <a href="/">Settings</a>
        </nav>
      </aside>

      <section className="content">
        <header className="hero">
          <div>
            <p className="eyebrow">Server Internet Usage</p>
            <h1>Live Debian server traffic accounting</h1>
            <p>Public Internet, LAN, and Docker/internal traffic are separated by the collector. Values refresh every 5 seconds.</p>
          </div>
          <StatusPill collector={collector} />
        </header>

        {error ? <section className="alert">{error}</section> : null}

        <section className="grid">
          <MetricCard label="Internet Download Today" value={formatBytes(today?.internet_download_bytes ?? 0)} detail="Public remote IP traffic" />
          <MetricCard label="Internet Upload Today" value={formatBytes(today?.internet_upload_bytes ?? 0)} detail={`Total Internet ${formatBytes(totalInternet)}`} />
          <MetricCard label="LAN Traffic Today" value={formatBytes(totalLAN)} detail={`${formatBytes(today?.lan_download_bytes ?? 0)} down / ${formatBytes(today?.lan_upload_bytes ?? 0)} up`} />
          <MetricCard label="Docker Internal" value={formatBytes((today?.docker_download_bytes ?? 0) + (today?.docker_upload_bytes ?? 0))} detail="Excluded from Internet totals" />
          <MetricCard label="Collector Samples" value={String(collector?.totals.samples_read ?? 0)} detail={`${collector?.totals.deltas_written ?? 0} deltas written`} />
          <MetricCard label="Accounting Mode" value={collector?.accounting ?? "Loading"} detail={collector?.last_error ?? "Healthy"} />
          <MetricCard label="Default Interface" value={collector?.route.default_interface ?? "Loading"} detail={`Gateway ${collector?.route.default_gateway ?? "unknown"}`} />
          <MetricCard label="Last Sample" value={formatDateTime(collector?.last_sample_at)} detail={updatedAt ? `UI updated ${updatedAt.toLocaleTimeString()}` : "Waiting"} />
        </section>

        <section className="panel">
          <div className="panel-title">
            <Wifi size={18} />
            <h2>Recent Hourly Traffic</h2>
          </div>
          <div className="table">
            <div className="table-row table-head">
              <span>Hour</span>
              <span>Class</span>
              <span>Download</span>
              <span>Upload</span>
            </div>
            {recentHourly.length === 0 ? (
              <div className="empty">No hourly buckets recorded yet.</div>
            ) : (
              recentHourly.map((bucket) => (
                <div className="table-row" key={`${bucket.bucket_start}-${bucket.traffic_class}`}>
                  <span>{formatDateTime(bucket.bucket_start)}</span>
                  <span className={`badge ${bucket.traffic_class}`}>{bucket.traffic_class}</span>
                  <span>{formatBytes(bucket.download_bytes)}</span>
                  <span>{formatBytes(bucket.upload_bytes)}</span>
                </div>
              ))
            )}
          </div>
        </section>

        <section className="panel split">
          <div>
            <div className="panel-title">
              <Database size={18} />
              <h2>Data Source</h2>
            </div>
            <p className="muted">Totals come from PostgreSQL aggregates written by the collector. Collector runtime totals reset when the collector restarts.</p>
          </div>
          <button className="refresh" type="button" onClick={() => void load()}>
            <RefreshCw size={16} />
            Refresh
          </button>
        </section>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
