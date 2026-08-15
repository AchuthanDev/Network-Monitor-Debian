package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type UsageTotals struct {
	InternetDownload uint64 `json:"internet_download_bytes"`
	InternetUpload   uint64 `json:"internet_upload_bytes"`
	LANDownload      uint64 `json:"lan_download_bytes"`
	LANUpload        uint64 `json:"lan_upload_bytes"`
	DockerDownload   uint64 `json:"docker_download_bytes"`
	DockerUpload     uint64 `json:"docker_upload_bytes"`
}

type HourlyBucket struct {
	BucketStart   time.Time `json:"bucket_start"`
	TrafficClass  string    `json:"traffic_class"`
	DownloadBytes uint64    `json:"download_bytes"`
	UploadBytes   uint64    `json:"upload_bytes"`
}

type DestinationRow struct {
	DestinationIP   string    `json:"destination_ip"`
	Domain          string    `json:"domain,omitempty"`
	Service         string    `json:"service"`
	Category        string    `json:"category"`
	Confidence      string    `json:"confidence"`
	DownloadBytes   uint64    `json:"download_bytes"`
	UploadBytes     uint64    `json:"upload_bytes"`
	ConnectionCount uint64    `json:"connection_count"`
	DeviceCount     uint64    `json:"device_count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
}

type DeviceReport struct {
	DeviceID        string    `json:"device_id"`
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
	InternetBytes   uint64    `json:"internet_bytes"`
	DownloadBytes   uint64    `json:"download_bytes"`
	UploadBytes     uint64    `json:"upload_bytes"`
	FreeBytes       uint64    `json:"free_night_bytes"`
	AnytimeBytes    uint64    `json:"anytime_bytes"`
	LANBytes        uint64    `json:"lan_bytes"`
	PeakHour        string    `json:"peak_hour,omitempty"`
	TopService      string    `json:"top_service,omitempty"`
	TopCategory     string    `json:"top_category,omitempty"`
	TopDestination  string    `json:"top_destination,omitempty"`
	ConnectionCount uint64    `json:"connection_count"`
}

type CategoryBreakdownRow struct {
	Category      string `json:"category"`
	Service       string `json:"service"`
	Confidence    string `json:"confidence"`
	DownloadBytes uint64 `json:"download_bytes"`
	UploadBytes   uint64 `json:"upload_bytes"`
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("database url is empty")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) TodayTotals(ctx context.Context, now time.Time) (UsageTotals, error) {
	start := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC)
	rows, err := s.pool.Query(ctx, `
		SELECT traffic_class, COALESCE(SUM(rx_bytes), 0), COALESCE(SUM(tx_bytes), 0)
		FROM traffic_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2
		GROUP BY traffic_class
	`, start, start.Add(24*time.Hour))
	if err != nil {
		return UsageTotals{}, err
	}
	defer rows.Close()

	var totals UsageTotals
	for rows.Next() {
		var class string
		var rx uint64
		var tx uint64
		if err := rows.Scan(&class, &rx, &tx); err != nil {
			return UsageTotals{}, err
		}
		switch class {
		case "internet":
			totals.InternetDownload = rx
			totals.InternetUpload = tx
		case "lan":
			totals.LANDownload = rx
			totals.LANUpload = tx
		case "docker_internal":
			totals.DockerDownload = rx
			totals.DockerUpload = tx
		}
	}
	return totals, rows.Err()
}

func (s *Store) HasTrafficData(ctx context.Context) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM traffic_samples LIMIT 1)`).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *Store) Hourly(ctx context.Context, from time.Time, to time.Time) ([]HourlyBucket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT bucket_start, traffic_class, rx_bytes, tx_bytes
		FROM traffic_hourly
		WHERE bucket_start >= $1 AND bucket_start < $2
		ORDER BY bucket_start ASC, traffic_class ASC
	`, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := make([]HourlyBucket, 0)
	for rows.Next() {
		var bucket HourlyBucket
		if err := rows.Scan(&bucket.BucketStart, &bucket.TrafficClass, &bucket.DownloadBytes, &bucket.UploadBytes); err != nil {
			return nil, err
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}

func (s *Store) Destinations(ctx context.Context, from time.Time, to time.Time) ([]DestinationRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT destination_ip::text, COALESCE(MAX(domain), ''), service, category, confidence,
		       COALESCE(SUM(download_bytes), 0), COALESCE(SUM(upload_bytes), 0),
		       COALESCE(SUM(connection_count), 0), COUNT(DISTINCT device_id),
		       MIN(first_seen), MAX(last_seen)
		FROM destination_usage_hour
		WHERE bucket_start >= $1 AND bucket_start < $2
		GROUP BY destination_ip, service, category, confidence
		ORDER BY COALESCE(SUM(download_bytes + upload_bytes), 0) DESC
		LIMIT 100
	`, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DestinationRow
	for rows.Next() {
		var row DestinationRow
		if err := rows.Scan(&row.DestinationIP, &row.Domain, &row.Service, &row.Category, &row.Confidence, &row.DownloadBytes, &row.UploadBytes, &row.ConnectionCount, &row.DeviceCount, &row.FirstSeen, &row.LastSeen); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *Store) DeviceReport(ctx context.Context, deviceID string, from time.Time, to time.Time) (DeviceReport, []CategoryBreakdownRow, error) {
	report := DeviceReport{DeviceID: deviceID, From: from.UTC(), To: to.UTC()}
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(internet_download_bytes), 0),
		       COALESCE(SUM(internet_upload_bytes), 0),
		       COALESCE(SUM(lan_download_bytes + lan_upload_bytes), 0),
		       COALESCE(SUM(CASE WHEN isp_period = 'free_night' THEN internet_download_bytes + internet_upload_bytes ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN isp_period = 'anytime' THEN internet_download_bytes + internet_upload_bytes ELSE 0 END), 0)
		FROM device_traffic_hour
		WHERE device_id = $1 AND bucket_start >= $2 AND bucket_start < $3
	`, deviceID, from.UTC(), to.UTC()).Scan(&report.DownloadBytes, &report.UploadBytes, &report.LANBytes, &report.FreeBytes, &report.AnytimeBytes)
	if err != nil {
		return report, nil, err
	}
	report.InternetBytes = report.DownloadBytes + report.UploadBytes

	_ = s.pool.QueryRow(ctx, `
		SELECT to_char(bucket_start, 'HH24:00') AS hour
		FROM device_traffic_hour
		WHERE device_id = $1 AND bucket_start >= $2 AND bucket_start < $3
		GROUP BY bucket_start
		ORDER BY SUM(internet_download_bytes + internet_upload_bytes) DESC
		LIMIT 1
	`, deviceID, from.UTC(), to.UTC()).Scan(&report.PeakHour)

	categoryRows, err := s.CategoryBreakdown(ctx, deviceID, from, to)
	if err != nil {
		return report, nil, err
	}
	if len(categoryRows) > 0 {
		report.TopCategory = categoryRows[0].Category
		report.TopService = categoryRows[0].Service
	}
	_ = s.pool.QueryRow(ctx, `
		SELECT destination_ip::text, COALESCE(SUM(connection_count), 0)
		FROM destination_usage_hour
		WHERE device_id = $1 AND bucket_start >= $2 AND bucket_start < $3
		GROUP BY destination_ip
		ORDER BY SUM(download_bytes + upload_bytes) DESC
		LIMIT 1
	`, deviceID, from.UTC(), to.UTC()).Scan(&report.TopDestination, &report.ConnectionCount)
	return report, categoryRows, nil
}

func (s *Store) CategoryBreakdown(ctx context.Context, deviceID string, from time.Time, to time.Time) ([]CategoryBreakdownRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT category, service, classification_confidence,
		       COALESCE(SUM(rx_bytes), 0), COALESCE(SUM(tx_bytes), 0)
		FROM device_category_usage
		WHERE device_id = $1 AND bucket_start >= $2 AND bucket_start < $3
		GROUP BY category, service, classification_confidence
		ORDER BY COALESCE(SUM(rx_bytes + tx_bytes), 0) DESC
	`, deviceID, from.UTC(), to.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CategoryBreakdownRow
	for rows.Next() {
		var row CategoryBreakdownRow
		if err := rows.Scan(&row.Category, &row.Service, &row.Confidence, &row.DownloadBytes, &row.UploadBytes); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
