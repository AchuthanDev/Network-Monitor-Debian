CREATE TABLE IF NOT EXISTS devices (
  id text PRIMARY KEY,
  mac_address macaddr UNIQUE,
  current_ip inet,
  hostname text,
  friendly_name text,
  device_type text,
  manufacturer text,
  first_seen timestamptz NOT NULL,
  last_seen timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'unknown'
);

CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices (last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_devices_current_ip ON devices (current_ip);

CREATE TABLE IF NOT EXISTS device_observations (
  id bigserial PRIMARY KEY,
  device_id text REFERENCES devices(id) ON DELETE CASCADE,
  observed_at timestamptz NOT NULL,
  mac_address macaddr,
  ip inet,
  hostname text,
  manufacturer text,
  source text NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_device_observations_device_time ON device_observations (device_id, observed_at DESC);
CREATE INDEX IF NOT EXISTS idx_device_observations_ip_time ON device_observations (ip, observed_at DESC);

CREATE TABLE IF NOT EXISTS device_traffic_minute (
  bucket_start timestamptz NOT NULL,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  traffic_class text NOT NULL,
  isp_period text NOT NULL,
  internet_download_bytes bigint NOT NULL DEFAULT 0 CHECK (internet_download_bytes >= 0),
  internet_upload_bytes bigint NOT NULL DEFAULT 0 CHECK (internet_upload_bytes >= 0),
  lan_download_bytes bigint NOT NULL DEFAULT 0 CHECK (lan_download_bytes >= 0),
  lan_upload_bytes bigint NOT NULL DEFAULT 0 CHECK (lan_upload_bytes >= 0),
  PRIMARY KEY (bucket_start, device_id, traffic_class, isp_period)
);

CREATE INDEX IF NOT EXISTS idx_device_traffic_minute_device_time ON device_traffic_minute (device_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_device_traffic_minute_time ON device_traffic_minute (bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_device_traffic_minute_period_time ON device_traffic_minute (isp_period, bucket_start DESC);

CREATE TABLE IF NOT EXISTS device_traffic_hour (
  bucket_start timestamptz NOT NULL,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  traffic_class text NOT NULL,
  isp_period text NOT NULL,
  internet_download_bytes bigint NOT NULL DEFAULT 0 CHECK (internet_download_bytes >= 0),
  internet_upload_bytes bigint NOT NULL DEFAULT 0 CHECK (internet_upload_bytes >= 0),
  lan_download_bytes bigint NOT NULL DEFAULT 0 CHECK (lan_download_bytes >= 0),
  lan_upload_bytes bigint NOT NULL DEFAULT 0 CHECK (lan_upload_bytes >= 0),
  PRIMARY KEY (bucket_start, device_id, traffic_class, isp_period)
);

CREATE INDEX IF NOT EXISTS idx_device_traffic_hour_device_time ON device_traffic_hour (device_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_device_traffic_hour_top_usage ON device_traffic_hour (bucket_start DESC, internet_download_bytes DESC, internet_upload_bytes DESC);

CREATE TABLE IF NOT EXISTS device_traffic_day (
  bucket_date date NOT NULL,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  traffic_class text NOT NULL,
  isp_period text NOT NULL,
  internet_download_bytes bigint NOT NULL DEFAULT 0 CHECK (internet_download_bytes >= 0),
  internet_upload_bytes bigint NOT NULL DEFAULT 0 CHECK (internet_upload_bytes >= 0),
  lan_download_bytes bigint NOT NULL DEFAULT 0 CHECK (lan_download_bytes >= 0),
  lan_upload_bytes bigint NOT NULL DEFAULT 0 CHECK (lan_upload_bytes >= 0),
  PRIMARY KEY (bucket_date, device_id, traffic_class, isp_period)
);

CREATE INDEX IF NOT EXISTS idx_device_traffic_day_device_date ON device_traffic_day (device_id, bucket_date DESC);
CREATE INDEX IF NOT EXISTS idx_device_traffic_day_period_date ON device_traffic_day (isp_period, bucket_date DESC);
CREATE INDEX IF NOT EXISTS idx_device_traffic_day_top_usage ON device_traffic_day (bucket_date DESC, internet_download_bytes DESC, internet_upload_bytes DESC);

CREATE TABLE IF NOT EXISTS device_destinations (
  bucket_start timestamptz NOT NULL,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  destination_ip inet NOT NULL,
  hostname text,
  protocol text NOT NULL,
  classification_confidence text NOT NULL DEFAULT 'unknown',
  rx_bytes bigint NOT NULL DEFAULT 0 CHECK (rx_bytes >= 0),
  tx_bytes bigint NOT NULL DEFAULT 0 CHECK (tx_bytes >= 0),
  PRIMARY KEY (bucket_start, device_id, destination_ip, protocol)
);

CREATE INDEX IF NOT EXISTS idx_device_destinations_device_time ON device_destinations (device_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_device_destinations_destination_time ON device_destinations (destination_ip, bucket_start DESC);

CREATE TABLE IF NOT EXISTS device_category_usage (
  bucket_start timestamptz NOT NULL,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  category text NOT NULL,
  service text NOT NULL DEFAULT 'unknown',
  classification_confidence text NOT NULL DEFAULT 'unknown',
  rx_bytes bigint NOT NULL DEFAULT 0 CHECK (rx_bytes >= 0),
  tx_bytes bigint NOT NULL DEFAULT 0 CHECK (tx_bytes >= 0),
  PRIMARY KEY (bucket_start, device_id, category, service, classification_confidence)
);

CREATE INDEX IF NOT EXISTS idx_device_category_usage_device_time ON device_category_usage (device_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_device_category_usage_category_time ON device_category_usage (category, bucket_start DESC);
