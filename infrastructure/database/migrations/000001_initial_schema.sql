CREATE TABLE IF NOT EXISTS schema_migrations (
  version bigint PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS settings (
  key text PRIMARY KEY,
  value jsonb NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS traffic_samples (
  id bigserial PRIMARY KEY,
  sampled_at timestamptz NOT NULL,
  source_type text NOT NULL,
  source_id text,
  process_pid integer,
  container_id text,
  local_ip inet,
  local_port integer,
  remote_ip inet,
  remote_port integer,
  protocol text NOT NULL,
  traffic_class text NOT NULL,
  rx_bytes bigint NOT NULL CHECK (rx_bytes >= 0),
  tx_bytes bigint NOT NULL CHECK (tx_bytes >= 0),
  attribution_confidence text NOT NULL DEFAULT 'unknown'
);

CREATE INDEX IF NOT EXISTS idx_traffic_samples_time_class ON traffic_samples (sampled_at, traffic_class);
CREATE INDEX IF NOT EXISTS idx_traffic_samples_container_time ON traffic_samples (container_id, sampled_at);
CREATE INDEX IF NOT EXISTS idx_traffic_samples_process_time ON traffic_samples (process_pid, sampled_at);
CREATE INDEX IF NOT EXISTS idx_traffic_samples_remote_time ON traffic_samples (remote_ip, sampled_at);

CREATE TABLE IF NOT EXISTS traffic_minute (
  bucket_start timestamptz NOT NULL,
  source_type text NOT NULL,
  source_id text NOT NULL,
  traffic_class text NOT NULL,
  rx_bytes bigint NOT NULL CHECK (rx_bytes >= 0),
  tx_bytes bigint NOT NULL CHECK (tx_bytes >= 0),
  PRIMARY KEY (bucket_start, source_type, source_id, traffic_class)
);

CREATE TABLE IF NOT EXISTS traffic_hourly (
  bucket_start timestamptz NOT NULL,
  source_type text NOT NULL,
  source_id text NOT NULL,
  traffic_class text NOT NULL,
  rx_bytes bigint NOT NULL CHECK (rx_bytes >= 0),
  tx_bytes bigint NOT NULL CHECK (tx_bytes >= 0),
  PRIMARY KEY (bucket_start, source_type, source_id, traffic_class)
);

CREATE TABLE IF NOT EXISTS traffic_daily (
  bucket_date date NOT NULL,
  source_type text NOT NULL,
  source_id text NOT NULL,
  traffic_class text NOT NULL,
  rx_bytes bigint NOT NULL CHECK (rx_bytes >= 0),
  tx_bytes bigint NOT NULL CHECK (tx_bytes >= 0),
  PRIMARY KEY (bucket_date, source_type, source_id, traffic_class)
);

CREATE TABLE IF NOT EXISTS alerts (
  id bigserial PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now(),
  severity text NOT NULL,
  title text NOT NULL,
  body text NOT NULL,
  acknowledged_at timestamptz
);
