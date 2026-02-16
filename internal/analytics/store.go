// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package analytics

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Summary represents an aggregated flow in a time bucket
type Summary struct {
	BucketTime time.Time `json:"bucket_time"`
	SrcMAC     string    `json:"src_mac"`
	SrcIP      string    `json:"src_ip"`
	DstIP      string    `json:"dst_ip"`
	DstPort    int       `json:"dst_port"`
	Protocol   string    `json:"protocol"`
	Bytes      int64     `json:"bytes"`
	Packets    int64     `json:"packets"`
	Class      string    `json:"class"`
}

// Store handles persistence of analytics data to SQLite
type Store struct {
	db *sql.DB
}

// Open opens or creates the analytics database
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open analytics db: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS flow_summaries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bucket_time INTEGER NOT NULL, -- Unix timestamp
		src_mac TEXT NOT NULL,
		src_ip TEXT,
		dst_ip TEXT,
		dst_port INTEGER,
		proto TEXT,
		bytes INTEGER DEFAULT 0,
		packets INTEGER DEFAULT 0,
		class TEXT,
		UNIQUE(bucket_time, src_mac, src_ip, dst_ip, dst_port, proto)
	);
	CREATE INDEX IF NOT EXISTS idx_flow_summaries_time ON flow_summaries(bucket_time);
	CREATE INDEX IF NOT EXISTS idx_flow_summaries_device ON flow_summaries(src_mac);
	`
	_, err := s.db.Exec(schema)
	return err
}

// RecordSummaries persists a batch of flow summaries using UPSERT
func (s *Store) RecordSummaries(summaries []Summary) error {
	if len(summaries) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO flow_summaries (bucket_time, src_mac, src_ip, dst_ip, dst_port, proto, bytes, packets, class)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_time, src_mac, src_ip, dst_ip, dst_port, proto) DO UPDATE SET
			bytes = bytes + excluded.bytes,
			packets = packets + excluded.packets,
			class = CASE WHEN excluded.class != '' THEN excluded.class ELSE class END
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, sum := range summaries {
		_, err := stmt.Exec(
			sum.BucketTime.Unix(),
			sum.SrcMAC,
			sum.SrcIP,
			sum.DstIP,
			sum.DstPort,
			sum.Protocol,
			sum.Bytes,
			sum.Packets,
			sum.Class,
		)
		if err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

// GetBandwidthUsage returns aggregated bytes per second in a time range for a device or zone
func (s *Store) GetBandwidthUsage(srcMAC string, from, to time.Time) ([]struct {
	Time  time.Time `json:"time"`
	Bytes int64     `json:"bytes"`
}, error) {
	query := `
		SELECT bucket_time, SUM(bytes)
		FROM flow_summaries
		WHERE bucket_time >= ? AND bucket_time <= ?
	`
	args := []interface{}{from.Unix(), to.Unix()}

	if srcMAC != "" {
		query += " AND src_mac = ?"
		args = append(args, srcMAC)
	}

	query += " GROUP BY bucket_time ORDER BY bucket_time ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []struct {
		Time  time.Time `json:"time"`
		Bytes int64     `json:"bytes"`
	}
	for rows.Next() {
		var ts int64
		var b int64
		if err := rows.Scan(&ts, &b); err != nil {
			return nil, err
		}
		result = append(result, struct {
			Time  time.Time `json:"time"`
			Bytes int64     `json:"bytes"`
		}{time.Unix(ts, 0), b})
	}
	return result, nil
}

// GetTopTalkers returns the top N devices by byte count in a time range
func (s *Store) GetTopTalkers(from, to time.Time, limit int) ([]Summary, error) {
	query := `
		SELECT src_mac, SUM(bytes), SUM(packets)
		FROM flow_summaries
		WHERE bucket_time >= ? AND bucket_time <= ?
		GROUP BY src_mac
		ORDER BY SUM(bytes) DESC
		LIMIT ?
	`
	rows, err := s.db.Query(query, from.Unix(), to.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Summary
	for rows.Next() {
		var sum Summary
		if err := rows.Scan(&sum.SrcMAC, &sum.Bytes, &sum.Packets); err != nil {
			return nil, err
		}
		result = append(result, sum)
	}
	return result, nil
}

// GetHistoricalFlows returns detailed flow summaries with filtering
func (s *Store) GetHistoricalFlows(srcMAC string, from, to time.Time, limit, offset int) ([]Summary, error) {
	query := `
		SELECT bucket_time, src_mac, src_ip, dst_ip, dst_port, proto, bytes, packets, class
		FROM flow_summaries
		WHERE bucket_time >= ? AND bucket_time <= ?
	`
	args := []interface{}{from.Unix(), to.Unix()}
	if srcMAC != "" {
		query += " AND src_mac = ?"
		args = append(args, srcMAC)
	}

	query += " ORDER BY bucket_time DESC, bytes DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Summary
	for rows.Next() {
		var sum Summary
		var ts int64
		err := rows.Scan(
			&ts, &sum.SrcMAC, &sum.SrcIP, &sum.DstIP, &sum.DstPort, &sum.Protocol,
			&sum.Bytes, &sum.Packets, &sum.Class,
		)
		if err != nil {
			return nil, err
		}
		sum.BucketTime = time.Unix(ts, 0)
		result = append(result, sum)
	}
	return result, nil
}

// Cleanup removes records older than the retention period
func (s *Store) Cleanup(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).Unix()
	result, err := s.db.Exec("DELETE FROM flow_summaries WHERE bucket_time < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
