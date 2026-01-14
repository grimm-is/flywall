package querylog

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store handles persistence of DNS query logs to SQLite
type Store struct {
	db *sql.DB
}

// Open opens or creates the query log database
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open querylog db: %w", err)
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
	CREATE TABLE IF NOT EXISTS query_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp INTEGER NOT NULL, -- Unix timestamp
		client_ip TEXT NOT NULL,
		domain TEXT NOT NULL,
		type TEXT,
		rcode TEXT,
		upstream TEXT,
		duration_ms INTEGER,
		blocked BOOLEAN,
		blocklist TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON query_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_logs_domain ON query_logs(domain);
	CREATE INDEX IF NOT EXISTS idx_logs_client ON query_logs(client_ip);
	`
	_, err := s.db.Exec(schema)
	return err
}

// RecordEntry persists a single query log entry (or batch if generalized later)
func (s *Store) RecordEntry(e Entry) error {
	query := `
		INSERT INTO query_logs (timestamp, client_ip, domain, type, rcode, upstream, duration_ms, blocked, blocklist)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		e.Timestamp.Unix(),
		e.ClientIP,
		e.Domain,
		e.Type,
		e.RCode,
		e.Upstream,
		e.DurationMs,
		e.Blocked,
		e.BlockList,
	)
	return err
}

// GetRecentLogs returns recent logs with optional filtering
func (s *Store) GetRecentLogs(limit int, offset int, search string) ([]Entry, error) {
	query := `
		SELECT timestamp, client_ip, domain, type, rcode, upstream, duration_ms, blocked, blocklist
		FROM query_logs
	`
	var args []interface{}

	if search != "" {
		query += " WHERE domain LIKE ? OR client_ip LIKE ?"
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
	}

	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []Entry
	for rows.Next() {
		var e Entry
		var ts int64
		err := rows.Scan(
			&ts, &e.ClientIP, &e.Domain, &e.Type, &e.RCode, &e.Upstream,
			&e.DurationMs, &e.Blocked, &e.BlockList,
		)
		if err != nil {
			return nil, err
		}
		e.Timestamp = time.Unix(ts, 0)
		logs = append(logs, e)
	}
	return logs, nil
}

// GetStats returns aggregated statistics for the given time range
func (s *Store) GetStats(from, to time.Time) (*Stats, error) {
	stats := &Stats{}

	// Total and Blocked
	err := s.db.QueryRow(`
		SELECT 
			COUNT(*), 
			SUM(CASE WHEN blocked THEN 1 ELSE 0 END)
		FROM query_logs
		WHERE timestamp >= ? AND timestamp <= ?
	`, from.Unix(), to.Unix()).Scan(&stats.TotalQueries, &stats.BlockedQueries)
	if err != nil {
		return nil, err
	}

	// Top Domains
	rows, err := s.db.Query(`
		SELECT domain, COUNT(*) as count
		FROM query_logs
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY domain
		ORDER BY count DESC
		LIMIT 10
	`, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ds DomainStat
		if err := rows.Scan(&ds.Domain, &ds.Count); err != nil {
			return nil, err
		}
		stats.TopDomains = append(stats.TopDomains, ds)
	}
	rows.Close()

	// Top Clients
	rows, err = s.db.Query(`
		SELECT client_ip, COUNT(*) as count
		FROM query_logs
		WHERE timestamp >= ? AND timestamp <= ?
		GROUP BY client_ip
		ORDER BY count DESC
		LIMIT 10
	`, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cs ClientStat
		if err := rows.Scan(&cs.ClientIP, &cs.Count); err != nil {
			return nil, err
		}
		stats.TopClients = append(stats.TopClients, cs)
	}
	rows.Close()

	// Top Blocked
	rows, err = s.db.Query(`
		SELECT domain, COUNT(*) as count
		FROM query_logs
		WHERE timestamp >= ? AND timestamp <= ? AND blocked = 1
		GROUP BY domain
		ORDER BY count DESC
		LIMIT 10
	`, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ds DomainStat
		if err := rows.Scan(&ds.Domain, &ds.Count); err != nil {
			return nil, err
		}
		stats.TopBlocked = append(stats.TopBlocked, ds)
	}

	return stats, nil
}

// Cleanup removes records older than retention period
func (s *Store) Cleanup(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).Unix()
	result, err := s.db.Exec("DELETE FROM query_logs WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
