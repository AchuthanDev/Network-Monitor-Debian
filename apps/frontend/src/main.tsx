import React from "react";
import { createRoot } from "react-dom/client";
import { Activity, AlertTriangle, Server } from "lucide-react";
import "./styles.css";

type MetricCardProps = {
  label: string;
  value: string;
  detail: string;
};

function MetricCard({ label, value, detail }: MetricCardProps) {
  return (
    <section className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </section>
  );
}

function App() {
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
            <h1>Server Internet accounting is collector-driven</h1>
            <p>
              The dashboard shows unavailable states whenever the collector cannot safely measure bytes. NIC totals are never used as Internet usage.
            </p>
          </div>
          <div className="status-pill">
            <AlertTriangle size={16} />
            Metrics unavailable
          </div>
        </header>

        <section className="grid">
          <MetricCard label="Internet Download Today" value="Unavailable" detail="Waiting for conntrack data" />
          <MetricCard label="Internet Upload Today" value="Unavailable" detail="No fake production data" />
          <MetricCard label="LAN Traffic Today" value="Unavailable" detail="Tracked separately" />
          <MetricCard label="Docker Internal" value="Unavailable" detail="Excluded from Internet totals" />
        </section>

        <section className="panel">
          <div className="panel-title">
            <Activity size={18} />
            <h2>Accounting Contract</h2>
          </div>
          <ul>
            <li>Public remote IP traffic counts as server Internet usage once.</li>
            <li>LAN, loopback, and Docker internal traffic never affect Internet quota.</li>
            <li>Phase 2 uses conntrack byte deltas; process/container attribution follows in later phases.</li>
          </ul>
        </section>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
