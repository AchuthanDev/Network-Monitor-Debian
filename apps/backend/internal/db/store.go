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

	var buckets []HourlyBucket
	for rows.Next() {
		var bucket HourlyBucket
		if err := rows.Scan(&bucket.BucketStart, &bucket.TrafficClass, &bucket.DownloadBytes, &bucket.UploadBytes); err != nil {
			return nil, err
		}
		buckets = append(buckets, bucket)
	}
	return buckets, rows.Err()
}
