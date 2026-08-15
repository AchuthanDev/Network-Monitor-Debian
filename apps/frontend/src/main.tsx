import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  AlertTriangle,
  BarChart3,
  Bell,
  CheckCircle2,
  Container,
  Cpu,
  Database,
  Download,
  FileText,
  Gauge,
  Globe2,
  HardDrive,
  ListTree,
  Network,
  PlugZap,
  RefreshCw,
  Server,
  Settings,
  ShieldAlert,
  SlidersHorizontal,
  Wifi,
} from "lucide-react";
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
  today?: UsageTotals;
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
  message?: string;
  data?: HourlyBucket[];
};

type EndpointState<T> = {
  data: T | null;
  error: string | null;
  status: "idle" | "ok" | "unavailable" | "error";
};

type SectionId =
  | "overview"
  | "network"
  | "containers"
  | "processes"
  | "connections"
  | "server"
  | "alerts"
  | "reports"
  | "settings";

type AlertItem = {
  id: string;
  severity: "critical" | "warning" | "info";
  title: string;
  body: string;
  source: string;
};

const emptyTotals: UsageTotals = {
  internet_download_bytes: 0,
  internet_upload_bytes: 0,
  lan_download_bytes: 0,
  lan_upload_bytes: 0,
  docker_download_bytes: 0,
  docker_upload_bytes: 0,
};

const sections: Array<{ id: SectionId; label: string; icon: React.ElementType }> = [
  { id: "overview", label: "Overview", icon: Gauge },
  { id: "network", label: "Network", icon: Network },
  { id: "containers", label: "Containers", icon: Container },
  { id: "processes", label: "Processes", icon: Cpu },
  { id: "connections", label: "Connections", icon: PlugZap },
  { id: "server", label: "Server", icon: Server },
  { id: "alerts", label: "Alerts", icon: Bell },
  { id: "reports", label: "Reports", icon: FileText },
  { id: "settings", label: "Settings", icon: Settings },
];

function freshEndpoint<T>(): EndpointState<T> {
  return { data: null, error: null, status: "idle" };
}

function MetricCard({
  label,
  value,
  detail,
  tone = "neutral",
}: {
  label: string;
  value: string;
  detail: string;
  tone?: "neutral" | "good" | "warning" | "accent";
}) {
  return (
    <section className={`metric-card ${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </section>
  );
}

function StatusPill({ collector }: { collector: CollectorResponse | null }) {
  const ok = collector?.status === "ok";
  const Icon = ok ? CheckCircle2 : AlertTriangle;
  return (
    <div className={`status-pill ${ok ? "ok" : "warning"}`}>
      <Icon size={16} />
      {collector ? `${collector.accounting} ${ok ? "active" : "degraded"}` : "Loading"}
    </div>
  );
}

function SectionTitle({
  icon: Icon,
  eyebrow,
  title,
  children,
}: {
  icon: React.ElementType;
  eyebrow: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <header className="section-title">
      <div className="title-icon">
        <Icon size={22} />
      </div>
      <div>
        <p className="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p>{children}</p>
      </div>
    </header>
  );
}

function PanelTitle({ icon: Icon, title }: { icon: React.ElementType; title: string }) {
  return (
    <div className="panel-title">
      <Icon size={18} />
      <h2>{title}</h2>
    </div>
  );
}

function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="empty-state">
      <ShieldAlert size={22} />
      <strong>{title}</strong>
      <span>{body}</span>
    </div>
  );
}

function ClassBadge({ value }: { value: string }) {
  const className = value.replaceAll("_", "-");
  return <span className={`badge ${className}`}>{value.replaceAll("_", " ")}</span>;
}

function TrafficBar({ label, bytes, max }: { label: string; bytes: number; max: number }) {
  const percent = max > 0 ? Math.max(4, Math.round((bytes / max) * 100)) : 0;
  return (
    <div className="traffic-bar">
      <div className="bar-label">
        <span>{label}</span>
        <strong>{formatBytes(bytes)}</strong>
      </div>
      <div className="bar-track">
        <div style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

function EndpointStatus({
  label,
  endpoint,
}: {
  label: string;
  endpoint: EndpointState<unknown>;
}) {
  const ok = endpoint.status === "ok";
  return (
    <div className={`endpoint ${ok ? "ok" : "warning"}`}>
      {ok ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
      <div>
        <strong>{label}</strong>
        <span>{endpoint.error ?? endpoint.status}</span>
      </div>
    </div>
  );
}

async function fetchEndpoint<T>(url: string): Promise<EndpointState<T>> {
  try {
    const response = await fetch(url);
    const data = (await response.json()) as T & { status?: string; message?: string };
    if (!response.ok) {
      return {
        data,
        error: data.message ?? `${url} returned ${response.status}`,
        status: response.status === 503 ? "unavailable" : "error",
      };
    }
    return {
      data,
      error: data.message ?? null,
      status: data.status === "ok" ? "ok" : "unavailable",
    };
  } catch (error) {
    return {
      data: null,
      error: error instanceof Error ? error.message : `Failed to load ${url}`,
      status: "error",
    };
  }
}

function useDashboardData(refreshMs: number) {
  const [dashboard, setDashboard] = useState<EndpointState<DashboardResponse>>(freshEndpoint);
  const [collector, setCollector] = useState<EndpointState<CollectorResponse>>(freshEndpoint);
  const [hourly, setHourly] = useState<EndpointState<HourlyResponse>>(freshEndpoint);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);

  async function load() {
    const [dashboardResult, collectorResult, hourlyResult] = await Promise.all([
      fetchEndpoint<DashboardResponse>("/api/v1/dashboard"),
      fetchEndpoint<CollectorResponse>("/collector/healthz"),
      fetchEndpoint<HourlyResponse>("/api/v1/network/hourly"),
    ]);
    setDashboard(dashboardResult);
    setCollector(collectorResult);
    setHourly(hourlyResult);
    setUpdatedAt(new Date());
  }

  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), refreshMs);
    return () => window.clearInterval(timer);
  }, [refreshMs]);

  return { dashboard, collector, hourly, updatedAt, load };
}

function buildAlerts(
  dashboard: EndpointState<DashboardResponse>,
  collector: EndpointState<CollectorResponse>,
  hourly: EndpointState<HourlyResponse>,
): AlertItem[] {
  const alerts: AlertItem[] = [];
  if (dashboard.status !== "ok") {
    alerts.push({
      id: "dashboard-unavailable",
      severity: "warning",
      title: "Dashboard metrics unavailable",
      body: dashboard.error ?? "No verified traffic samples are available yet.",
      source: "API",
    });
  }
  if (collector.status !== "ok" || collector.data?.status !== "ok") {
    alerts.push({
      id: "collector-degraded",
      severity: "critical",
      title: "Collector is not fully healthy",
      body: collector.data?.last_error ?? collector.error ?? "Collector health endpoint is unavailable.",
      source: "Collector",
    });
  }
  if (hourly.status !== "ok") {
    alerts.push({
      id: "hourly-unavailable",
      severity: "warning",
      title: "Hourly report data unavailable",
      body: hourly.error ?? "Hourly traffic buckets could not be loaded.",
      source: "Database",
    });
  }
  if (alerts.length === 0) {
    alerts.push({
      id: "system-normal",
      severity: "info",
      title: "No active alerts",
      body: "All currently wired monitoring endpoints are reporting normally.",
      source: "System",
    });
  }
  return alerts;
}

function totalBytes(totals: UsageTotals, prefix: "internet" | "lan" | "docker") {
  if (prefix === "internet") {
    return totals.internet_download_bytes + totals.internet_upload_bytes;
  }
  if (prefix === "lan") {
    return totals.lan_download_bytes + totals.lan_upload_bytes;
  }
  return totals.docker_download_bytes + totals.docker_upload_bytes;
}

function OverviewView({
  totals,
  collector,
  hourlyBuckets,
  updatedAt,
  onRefresh,
}: {
  totals: UsageTotals;
  collector: CollectorResponse | null;
  hourlyBuckets: HourlyBucket[];
  updatedAt: Date | null;
  onRefresh: () => void;
}) {
  const totalInternet = totalBytes(totals, "internet");
  const totalLAN = totalBytes(totals, "lan");
  const recentHourly = hourlyBuckets.slice(-8).reverse();

  return (
    <>
      <SectionTitle icon={Gauge} eyebrow="Server Internet Usage" title="Live Debian server traffic accounting">
        Public Internet, LAN, and Docker/internal traffic are separated by the collector. Values refresh on the configured interval.
      </SectionTitle>

      <section className="grid">
        <MetricCard label="Internet Download Today" value={formatBytes(totals.internet_download_bytes)} detail="Public remote IP traffic" tone="accent" />
        <MetricCard label="Internet Upload Today" value={formatBytes(totals.internet_upload_bytes)} detail={`Total Internet ${formatBytes(totalInternet)}`} tone="accent" />
        <MetricCard label="LAN Traffic Today" value={formatBytes(totalLAN)} detail={`${formatBytes(totals.lan_download_bytes)} down / ${formatBytes(totals.lan_upload_bytes)} up`} tone="good" />
        <MetricCard label="Docker Internal" value={formatBytes(totalBytes(totals, "docker"))} detail="Excluded from Internet totals" />
        <MetricCard label="Collector Samples" value={String(collector?.totals.samples_read ?? 0)} detail={`${collector?.totals.deltas_written ?? 0} deltas written`} />
        <MetricCard label="Accounting Mode" value={collector?.accounting ?? "Loading"} detail={collector?.last_error ?? "Healthy"} />
        <MetricCard label="Default Interface" value={collector?.route.default_interface ?? "Loading"} detail={`Gateway ${collector?.route.default_gateway ?? "unknown"}`} />
        <MetricCard label="Last Sample" value={formatDateTime(collector?.last_sample_at)} detail={updatedAt ? `UI updated ${updatedAt.toLocaleTimeString()}` : "Waiting"} />
      </section>

      <HourlyTrafficPanel buckets={recentHourly} />

      <section className="panel split">
        <div>
          <PanelTitle icon={Database} title="Data Source" />
          <p className="muted">Totals come from PostgreSQL aggregates written by the collector. Collector runtime totals reset when the collector restarts.</p>
        </div>
        <button className="refresh" type="button" onClick={onRefresh}>
          <RefreshCw size={16} />
          Refresh
        </button>
      </section>
    </>
  );
}

function HourlyTrafficPanel({ buckets }: { buckets: HourlyBucket[] }) {
  return (
    <section className="panel">
      <PanelTitle icon={Wifi} title="Recent Hourly Traffic" />
      <div className="table">
        <div className="table-row table-head">
          <span>Hour</span>
          <span>Class</span>
          <span>Download</span>
          <span>Upload</span>
        </div>
        {buckets.length === 0 ? (
          <EmptyState title="No hourly buckets recorded" body="The database has no verified hourly traffic rows for this range." />
        ) : (
          buckets.map((bucket) => (
            <div className="table-row" key={`${bucket.bucket_start}-${bucket.traffic_class}`}>
              <span>{formatDateTime(bucket.bucket_start)}</span>
              <ClassBadge value={bucket.traffic_class} />
              <span>{formatBytes(bucket.download_bytes)}</span>
              <span>{formatBytes(bucket.upload_bytes)}</span>
            </div>
          ))
        )}
      </div>
    </section>
  );
}

function NetworkView({ totals, hourlyBuckets }: { totals: UsageTotals; hourlyBuckets: HourlyBucket[] }) {
  const rows = [
    { label: "Internet", bytes: totalBytes(totals, "internet") },
    { label: "LAN", bytes: totalBytes(totals, "lan") },
    { label: "Docker internal", bytes: totalBytes(totals, "docker") },
  ];
  const max = Math.max(...rows.map((row) => row.bytes), 0);

  return (
    <>
      <SectionTitle icon={Network} eyebrow="Network" title="Traffic classes and hourly movement">
        This view separates public Internet, local LAN, and Docker/internal traffic so ISP usage investigations are not polluted by local chatter.
      </SectionTitle>
      <section className="grid compact">
        <MetricCard label="Internet Total" value={formatBytes(rows[0].bytes)} detail={`${formatBytes(totals.internet_download_bytes)} down / ${formatBytes(totals.internet_upload_bytes)} up`} tone="accent" />
        <MetricCard label="LAN Total" value={formatBytes(rows[1].bytes)} detail={`${formatBytes(totals.lan_download_bytes)} down / ${formatBytes(totals.lan_upload_bytes)} up`} tone="good" />
        <MetricCard label="Docker Internal" value={formatBytes(rows[2].bytes)} detail={`${formatBytes(totals.docker_download_bytes)} down / ${formatBytes(totals.docker_upload_bytes)} up`} />
      </section>
      <section className="panel">
        <PanelTitle icon={BarChart3} title="Class Breakdown" />
        <div className="bars">
          {rows.map((row) => (
            <TrafficBar key={row.label} label={row.label} bytes={row.bytes} max={max} />
          ))}
        </div>
      </section>
      <HourlyTrafficPanel buckets={hourlyBuckets.slice(-24).reverse()} />
    </>
  );
}

function ContainersView({ totals }: { totals: UsageTotals }) {
  return (
    <>
      <SectionTitle icon={Container} eyebrow="Containers" title="Docker traffic attribution">
        Docker/internal byte totals are tracked now. Per-container attribution is reserved for the next collector phase and remains visibly unavailable until it is verified.
      </SectionTitle>
      <section className="grid compact">
        <MetricCard label="Docker Download" value={formatBytes(totals.docker_download_bytes)} detail="Internal Docker receive bytes" />
        <MetricCard label="Docker Upload" value={formatBytes(totals.docker_upload_bytes)} detail="Internal Docker transmit bytes" />
        <MetricCard label="Attribution Status" value="Not enabled" detail="No container IDs are reported by the collector yet" tone="warning" />
      </section>
      <section className="panel">
        <PanelTitle icon={ListTree} title="Container Usage" />
        <div className="table">
          <div className="table-row container-row table-head">
            <span>Container</span>
            <span>Traffic class</span>
            <span>Download</span>
            <span>Upload</span>
          </div>
          <EmptyState title="No container rows available" body="The schema has container fields, but the current collector does not yet write per-container records." />
        </div>
      </section>
    </>
  );
}

function ProcessesView() {
  return (
    <>
      <SectionTitle icon={Cpu} eyebrow="Processes" title="Process-level traffic attribution">
        Process PID fields exist in storage, but the current accounting path does not yet claim process ownership for live flows.
      </SectionTitle>
      <section className="grid compact">
        <MetricCard label="Process Attribution" value="Pending" detail="No verified process mapping is active" tone="warning" />
        <MetricCard label="Storage Readiness" value="Ready" detail="traffic_samples.process_pid is indexed" tone="good" />
        <MetricCard label="Display Policy" value="No fake data" detail="Empty until the collector writes process rows" />
      </section>
      <section className="panel">
        <PanelTitle icon={Cpu} title="Process Usage" />
        <div className="table">
          <div className="table-row process-row table-head">
            <span>PID</span>
            <span>Process</span>
            <span>Download</span>
            <span>Upload</span>
          </div>
          <EmptyState title="No process rows available" body="Enable a verified process attribution collector before this table can show live process usage." />
        </div>
      </section>
    </>
  );
}

function ConnectionsView({ totals, hourlyBuckets }: { totals: UsageTotals; hourlyBuckets: HourlyBucket[] }) {
  return (
    <>
      <SectionTitle icon={PlugZap} eyebrow="Connections" title="Connection accounting summary">
        The collector samples connection counters and stores classed byte deltas. Raw remote endpoints are intentionally not displayed until row-level connection APIs are added.
      </SectionTitle>
      <section className="grid compact">
        <MetricCard label="Public Internet" value={formatBytes(totalBytes(totals, "internet"))} detail="Remote public IP traffic" tone="accent" />
        <MetricCard label="LAN" value={formatBytes(totalBytes(totals, "lan"))} detail="Local network traffic" tone="good" />
        <MetricCard label="Docker/Internal" value={formatBytes(totalBytes(totals, "docker"))} detail="Excluded from ISP usage" />
      </section>
      <HourlyTrafficPanel buckets={hourlyBuckets.slice(-12).reverse()} />
    </>
  );
}

function ServerView({
  dashboard,
  collector,
  hourly,
}: {
  dashboard: EndpointState<DashboardResponse>;
  collector: EndpointState<CollectorResponse>;
  hourly: EndpointState<HourlyResponse>;
}) {
  return (
    <>
      <SectionTitle icon={Server} eyebrow="Server" title="Runtime health and collection path">
        This view shows the API, collector, database-backed hourly data, and detected default route used by the collector.
      </SectionTitle>
      <section className="grid compact">
        <MetricCard label="Collector Status" value={collector.data?.status ?? collector.status} detail={collector.data?.last_error ?? collector.error ?? "Collector endpoint reachable"} tone={collector.status === "ok" ? "good" : "warning"} />
        <MetricCard label="Mode" value={collector.data?.mode ?? "Unavailable"} detail={`Accounting ${collector.data?.accounting ?? "unknown"}`} />
        <MetricCard label="Default Interface" value={collector.data?.route.default_interface ?? "Unavailable"} detail={`Gateway ${collector.data?.route.default_gateway ?? "unknown"}`} />
        <MetricCard label="Last Sample" value={formatDateTime(collector.data?.last_sample_at)} detail={`${collector.data?.totals.samples_read ?? 0} runtime samples`} />
      </section>
      <section className="panel">
        <PanelTitle icon={HardDrive} title="Endpoint Health" />
        <div className="endpoints">
          <EndpointStatus label="Dashboard API" endpoint={dashboard} />
          <EndpointStatus label="Collector health" endpoint={collector} />
          <EndpointStatus label="Hourly database query" endpoint={hourly} />
        </div>
      </section>
    </>
  );
}

function AlertsView({ alerts }: { alerts: AlertItem[] }) {
  return (
    <>
      <SectionTitle icon={Bell} eyebrow="Alerts" title="Active monitoring conditions">
        Alerts are derived from live endpoint and collector states. The database alerts table is present, but alert rules are not yet persisted.
      </SectionTitle>
      <section className="alerts-list">
        {alerts.map((alert) => (
          <article className={`alert-card ${alert.severity}`} key={alert.id}>
            <div>
              <span>{alert.source}</span>
              <strong>{alert.title}</strong>
              <p>{alert.body}</p>
            </div>
            <ClassBadge value={alert.severity} />
          </article>
        ))}
      </section>
    </>
  );
}

function ReportsView({
  totals,
  hourlyBuckets,
  generatedAt,
}: {
  totals: UsageTotals;
  hourlyBuckets: HourlyBucket[];
  generatedAt?: string;
}) {
  function exportReport() {
    const payload = {
      generated_at: new Date().toISOString(),
      dashboard_generated_at: generatedAt,
      today: totals,
      hourly: hourlyBuckets,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `network-monitor-report-${new Date().toISOString().slice(0, 10)}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
  }

  return (
    <>
      <SectionTitle icon={FileText} eyebrow="Reports" title="Traffic evidence export">
        Export the currently loaded dashboard and hourly data as JSON for troubleshooting, audits, or ISP usage comparison.
      </SectionTitle>
      <section className="grid compact">
        <MetricCard label="Report Window" value="Today" detail={generatedAt ? `Generated ${formatDateTime(generatedAt)}` : "Waiting for dashboard"} />
        <MetricCard label="Hourly Rows" value={String(hourlyBuckets.length)} detail="Rows currently loaded from the API" />
        <MetricCard label="Internet Total" value={formatBytes(totalBytes(totals, "internet"))} detail="Download plus upload" tone="accent" />
      </section>
      <section className="panel split">
        <div>
          <PanelTitle icon={Download} title="JSON Export" />
          <p className="muted">The export contains only verified data currently returned by the backend endpoints.</p>
        </div>
        <button className="refresh" type="button" onClick={exportReport}>
          <Download size={16} />
          Export
        </button>
      </section>
      <HourlyTrafficPanel buckets={hourlyBuckets.slice(-24).reverse()} />
    </>
  );
}

function SettingsView({
  refreshMs,
  setRefreshMs,
}: {
  refreshMs: number;
  setRefreshMs: (value: number) => void;
}) {
  return (
    <>
      <SectionTitle icon={Settings} eyebrow="Settings" title="Monitoring display settings">
        These controls affect the browser dashboard. Collector, firewall, route, and database settings remain deployment-controlled.
      </SectionTitle>
      <section className="panel">
        <PanelTitle icon={SlidersHorizontal} title="Refresh Interval" />
        <div className="setting-row">
          <label htmlFor="refresh-interval">Dashboard refresh</label>
          <select id="refresh-interval" value={refreshMs} onChange={(event) => setRefreshMs(Number(event.target.value))}>
            <option value={5000}>5 seconds</option>
            <option value={10000}>10 seconds</option>
            <option value={30000}>30 seconds</option>
            <option value={60000}>60 seconds</option>
          </select>
        </div>
      </section>
      <section className="panel">
        <PanelTitle icon={Globe2} title="Collection Scope" />
        <div className="scope-grid">
          <span>Public Internet</span>
          <strong>Collected</strong>
          <span>LAN traffic</span>
          <strong>Collected</strong>
          <span>Docker internal traffic</span>
          <strong>Collected as class totals</strong>
          <span>Per-container attribution</span>
          <strong>Pending</strong>
          <span>Per-process attribution</span>
          <strong>Pending</strong>
        </div>
      </section>
    </>
  );
}

function App() {
  const [activeSection, setActiveSection] = useState<SectionId>("overview");
  const [refreshMs, setRefreshMs] = useState(5000);
  const { dashboard, collector, hourly, updatedAt, load } = useDashboardData(refreshMs);

  const totals = dashboard.data?.today ?? emptyTotals;
  const hourlyBuckets = hourly.data?.data ?? [];
  const alerts = useMemo(() => buildAlerts(dashboard, collector, hourly), [dashboard, collector, hourly]);
  const currentSection = sections.find((section) => section.id === activeSection) ?? sections[0];

  return (
    <main className="shell">
      <aside className="sidebar">
        <div className="brand">
          <Server size={22} />
          <span>Network Monitor Debian</span>
        </div>
        <nav aria-label="Main sections">
          {sections.map(({ id, label, icon: Icon }) => (
            <button className={id === activeSection ? "active" : ""} type="button" key={id} onClick={() => setActiveSection(id)}>
              <Icon size={17} />
              <span>{label}</span>
            </button>
          ))}
        </nav>
      </aside>

      <section className="content">
        <div className="topbar">
          <div>
            <span>Current section</span>
            <strong>{currentSection.label}</strong>
          </div>
          <StatusPill collector={collector.data} />
        </div>

        {dashboard.error ? <section className="alert">{dashboard.error}</section> : null}

        {activeSection === "overview" ? (
          <OverviewView totals={totals} collector={collector.data} hourlyBuckets={hourlyBuckets} updatedAt={updatedAt} onRefresh={() => void load()} />
        ) : null}
        {activeSection === "network" ? <NetworkView totals={totals} hourlyBuckets={hourlyBuckets} /> : null}
        {activeSection === "containers" ? <ContainersView totals={totals} /> : null}
        {activeSection === "processes" ? <ProcessesView /> : null}
        {activeSection === "connections" ? <ConnectionsView totals={totals} hourlyBuckets={hourlyBuckets} /> : null}
        {activeSection === "server" ? <ServerView dashboard={dashboard} collector={collector} hourly={hourly} /> : null}
        {activeSection === "alerts" ? <AlertsView alerts={alerts} /> : null}
        {activeSection === "reports" ? <ReportsView totals={totals} hourlyBuckets={hourlyBuckets} generatedAt={dashboard.data?.generated_at} /> : null}
        {activeSection === "settings" ? <SettingsView refreshMs={refreshMs} setRefreshMs={setRefreshMs} /> : null}
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
