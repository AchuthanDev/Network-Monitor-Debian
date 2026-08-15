CREATE TABLE IF NOT EXISTS dns_observations (
  id bigserial PRIMARY KEY,
  observed_at timestamptz NOT NULL,
  client_ip inet NOT NULL,
  device_id text REFERENCES devices(id) ON DELETE SET NULL,
  query_domain text NOT NULL,
  resolved_ip inet,
  source text NOT NULL DEFAULT 'pihole',
  retained_until timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dns_observations_client_time ON dns_observations (client_ip, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_dns_observations_resolved_time ON dns_observations (resolved_ip, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_dns_observations_retention ON dns_observations (retained_until);

CREATE TABLE IF NOT EXISTS destination_usage_hour (
  bucket_start timestamptz NOT NULL,
  device_id text REFERENCES devices(id) ON DELETE CASCADE,
  destination_ip inet NOT NULL,
  domain text,
  protocol text NOT NULL,
  service text NOT NULL DEFAULT 'unknown',
  category text NOT NULL DEFAULT 'unknown',
  confidence text NOT NULL DEFAULT 'unknown',
  evidence jsonb NOT NULL DEFAULT '[]'::jsonb,
  download_bytes bigint NOT NULL DEFAULT 0 CHECK (download_bytes >= 0),
  upload_bytes bigint NOT NULL DEFAULT 0 CHECK (upload_bytes >= 0),
  connection_count bigint NOT NULL DEFAULT 0 CHECK (connection_count >= 0),
  first_seen timestamptz NOT NULL,
  last_seen timestamptz NOT NULL,
  PRIMARY KEY (bucket_start, device_id, destination_ip, protocol, service, category, confidence)
);

CREATE INDEX IF NOT EXISTS idx_destination_usage_hour_time ON destination_usage_hour (bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_destination_usage_hour_device_time ON destination_usage_hour (device_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_destination_usage_hour_top ON destination_usage_hour (bucket_start DESC, download_bytes DESC, upload_bytes DESC);
CREATE INDEX IF NOT EXISTS idx_destination_usage_hour_category ON destination_usage_hour (category, bucket_start DESC);

CREATE TABLE IF NOT EXISTS alert_events (
  id bigserial PRIMARY KEY,
  rule_name text NOT NULL,
  device_id text REFERENCES devices(id) ON DELETE SET NULL,
  severity text NOT NULL,
  category text,
  service text,
  threshold_bytes bigint NOT NULL CHECK (threshold_bytes >= 0),
  measured_bytes bigint NOT NULL CHECK (measured_bytes >= 0),
  period_start timestamptz NOT NULL,
  period_end timestamptz NOT NULL,
  fired_at timestamptz NOT NULL,
  dedupe_key text NOT NULL,
  message text NOT NULL,
  evidence jsonb NOT NULL DEFAULT '[]'::jsonb
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_alert_events_dedupe ON alert_events (dedupe_key);
CREATE INDEX IF NOT EXISTS idx_alert_events_device_time ON alert_events (device_id, fired_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_events_time ON alert_events (fired_at DESC);

CREATE TABLE IF NOT EXISTS retention_policy (
  name text PRIMARY KEY,
  duration interval NOT NULL
);

INSERT INTO retention_policy (name, duration) VALUES
  ('dns_observations', interval '30 days'),
  ('destination_usage_hour', interval '1 year'),
  ('device_traffic_minute', interval '30 days'),
  ('device_traffic_hour', interval '1 year'),
  ('device_traffic_day', interval '10 years')
ON CONFLICT (name) DO NOTHING;
