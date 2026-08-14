package db

import (
	"context"
	"fmt"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/accounting"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Writer struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Writer, error) {
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
	return &Writer{pool: pool}, nil
}

func (w *Writer) Close() {
	w.pool.Close()
}

func (w *Writer) WriteDeltas(ctx context.Context, deltas []accounting.TrafficDelta) error {
	if len(deltas) == 0 {
		return nil
	}

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, delta := range deltas {
		if _, err := tx.Exec(ctx, `
			INSERT INTO traffic_samples (
				sampled_at, source_type, source_id, local_ip, local_port, remote_ip, remote_port,
				protocol, traffic_class, rx_bytes, tx_bytes, attribution_confidence
			)
			VALUES ($1, 'host', 'server', $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`,
			delta.ObservedAt,
			delta.LocalIP.String(),
			int(delta.LocalPort),
			delta.RemoteIP.String(),
			int(delta.RemotePort),
			delta.Protocol,
			string(delta.Class),
			int64(delta.DownloadBytes),
			int64(delta.UploadBytes),
			delta.AttributionConfidence,
		); err != nil {
			return err
		}

		if err := upsertAggregate(ctx, tx, "traffic_minute", delta.ObservedAt.Truncate(time.Minute), delta); err != nil {
			return err
		}
		if err := upsertAggregate(ctx, tx, "traffic_hourly", delta.ObservedAt.Truncate(time.Hour), delta); err != nil {
			return err
		}
		if err := upsertDaily(ctx, tx, delta); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

type executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func upsertAggregate(ctx context.Context, tx executor, table string, bucket time.Time, delta accounting.TrafficDelta) error {
	_, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (bucket_start, source_type, source_id, traffic_class, rx_bytes, tx_bytes)
		VALUES ($1, 'host', 'server', $2, $3, $4)
		ON CONFLICT (bucket_start, source_type, source_id, traffic_class)
		DO UPDATE SET
			rx_bytes = %s.rx_bytes + EXCLUDED.rx_bytes,
			tx_bytes = %s.tx_bytes + EXCLUDED.tx_bytes
	`, table, table, table),
		bucket,
		string(delta.Class),
		int64(delta.DownloadBytes),
		int64(delta.UploadBytes),
	)
	return err
}

func upsertDaily(ctx context.Context, tx executor, delta accounting.TrafficDelta) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO traffic_daily (bucket_date, source_type, source_id, traffic_class, rx_bytes, tx_bytes)
		VALUES ($1, 'host', 'server', $2, $3, $4)
		ON CONFLICT (bucket_date, source_type, source_id, traffic_class)
		DO UPDATE SET
			rx_bytes = traffic_daily.rx_bytes + EXCLUDED.rx_bytes,
			tx_bytes = traffic_daily.tx_bytes + EXCLUDED.tx_bytes
	`,
		delta.ObservedAt.UTC().Format("2006-01-02"),
		string(delta.Class),
		int64(delta.DownloadBytes),
		int64(delta.UploadBytes),
	)
	return err
}
