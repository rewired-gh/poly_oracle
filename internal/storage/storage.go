// Package storage provides SQLite-backed persistence for markets, snapshots, and changes.
// It uses modernc.org/sqlite (pure Go, no CGO) with WAL mode for concurrent reads.
package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
	_ "modernc.org/sqlite"
)

// Storage wraps a SQLite database for all persistence operations.
type Storage struct {
	db                   *sql.DB
	maxMarkets           int
	maxSnapshotsPerEvent int
}

// New opens (or creates) the SQLite database at dbPath.
// If dbPath is empty, defaults to $TMPDIR/polyoracle/data.db.
func New(maxMarkets, maxSnapshotsPerEvent int, dbPath string) (*Storage, error) {
	if dbPath == "" {
		dbPath = filepath.Join(os.TempDir(), "polyoracle", "data.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	// Single writer connection; WAL lets readers not block the writer.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	s := &Storage{db: db, maxMarkets: maxMarkets, maxSnapshotsPerEvent: maxSnapshotsPerEvent}
	if err := s.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}

// Save is a no-op: SQLite writes are immediate.
func (s *Storage) Save() error { return nil }

// Load is a no-op: SQLite data is always present on open.
func (s *Storage) Load() error { return nil }

func (s *Storage) createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS markets (
			id              TEXT PRIMARY KEY,
			event_id        TEXT NOT NULL,
			market_id       TEXT NOT NULL,
			market_question TEXT,
			title           TEXT NOT NULL,
			event_url       TEXT,
			description     TEXT,
			category        TEXT NOT NULL,
			subcategory     TEXT,
			yes_prob        REAL NOT NULL,
			no_prob         REAL NOT NULL,
			volume_24hr     REAL,
			volume_1wk      REAL,
			volume_1mo      REAL,
			liquidity       REAL,
			active          INTEGER,
			closed          INTEGER,
			last_updated    INTEGER NOT NULL,
			created_at      INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id        TEXT PRIMARY KEY,
			market_id TEXT NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
			yes_prob  REAL NOT NULL,
			no_prob   REAL NOT NULL,
			timestamp INTEGER NOT NULL,
			source    TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_market_ts ON snapshots(market_id, timestamp)`,
		`CREATE TABLE IF NOT EXISTS changes (
			id                   TEXT PRIMARY KEY,
			market_id            TEXT NOT NULL,
			original_event_id    TEXT,
			event_title          TEXT,
			event_url            TEXT,
			polymarket_market_id TEXT,
			market_question      TEXT,
			magnitude            REAL NOT NULL,
			direction            TEXT NOT NULL,
			old_prob             REAL NOT NULL,
			new_prob             REAL NOT NULL,
			time_window          INTEGER NOT NULL,
			detected_at          INTEGER NOT NULL,
			notified             INTEGER DEFAULT 0,
			signal_score         REAL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_changes_detected_at ON changes(detected_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// --- Markets ---

func (s *Storage) AddMarket(market *models.Market) error {
	if err := market.Validate(); err != nil {
		return fmt.Errorf("invalid market: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(`
		INSERT INTO markets
			(id, event_id, market_id, market_question, title, event_url, description,
			 category, subcategory, yes_prob, no_prob, volume_24hr, volume_1wk, volume_1mo,
			 liquidity, active, closed, last_updated, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		market.ID, market.EventID, market.MarketID, market.MarketQuestion, market.Title,
		market.EventURL, market.Description, market.Category, market.Subcategory,
		market.YesProbability, market.NoProbability,
		market.Volume24hr, market.Volume1wk, market.Volume1mo, market.Liquidity,
		boolToInt(market.Active), boolToInt(market.Closed),
		market.LastUpdated.UnixNano(), market.CreatedAt.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert market: %w", err)
	}

	// Evict oldest market(s) if we exceed the cap (cascades to snapshots).
	if _, err = tx.Exec(`
		DELETE FROM markets WHERE id NOT IN (
			SELECT id FROM markets ORDER BY last_updated DESC LIMIT ?
		)`, s.maxMarkets); err != nil {
		return fmt.Errorf("failed to enforce market cap: %w", err)
	}

	return tx.Commit()
}

func (s *Storage) GetMarket(id string) (*models.Market, error) {
	row := s.db.QueryRow(`SELECT `+marketCols+` FROM markets WHERE id = ?`, id)
	m, err := scanMarket(row.Scan)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("market not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get market: %w", err)
	}
	return m, nil
}

func (s *Storage) GetAllMarkets() ([]*models.Market, error) {
	rows, err := s.db.Query(`SELECT ` + marketCols + ` FROM markets`)
	if err != nil {
		return nil, fmt.Errorf("failed to query markets: %w", err)
	}
	defer rows.Close()
	var markets []*models.Market
	for rows.Next() {
		m, err := scanMarket(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan market: %w", err)
		}
		markets = append(markets, m)
	}
	if markets == nil {
		markets = []*models.Market{}
	}
	return markets, rows.Err()
}

func (s *Storage) UpdateMarket(market *models.Market) error {
	if err := market.Validate(); err != nil {
		return fmt.Errorf("invalid market: %w", err)
	}
	res, err := s.db.Exec(`
		UPDATE markets SET
			event_id=?, market_id=?, market_question=?, title=?, event_url=?, description=?,
			category=?, subcategory=?, yes_prob=?, no_prob=?, volume_24hr=?, volume_1wk=?,
			volume_1mo=?, liquidity=?, active=?, closed=?, last_updated=?, created_at=?
		WHERE id=?`,
		market.EventID, market.MarketID, market.MarketQuestion, market.Title,
		market.EventURL, market.Description, market.Category, market.Subcategory,
		market.YesProbability, market.NoProbability,
		market.Volume24hr, market.Volume1wk, market.Volume1mo, market.Liquidity,
		boolToInt(market.Active), boolToInt(market.Closed),
		market.LastUpdated.UnixNano(), market.CreatedAt.UnixNano(),
		market.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update market: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("market not found: %s", market.ID)
	}
	return nil
}

// --- Snapshots ---

func (s *Storage) AddSnapshot(snapshot *models.Snapshot) error {
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("invalid snapshot: %w", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM markets WHERE id = ?`, snapshot.EventID).Scan(&count); err != nil {
		return fmt.Errorf("failed to verify market: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("market not found: %s", snapshot.EventID)
	}
	_, err := s.db.Exec(`
		INSERT INTO snapshots (id, market_id, yes_prob, no_prob, timestamp, source)
		VALUES (?,?,?,?,?,?)`,
		snapshot.ID, snapshot.EventID,
		snapshot.YesProbability, snapshot.NoProbability,
		snapshot.Timestamp.UnixNano(), snapshot.Source,
	)
	if err != nil {
		return fmt.Errorf("failed to insert snapshot: %w", err)
	}
	return nil
}

func (s *Storage) GetSnapshots(marketID string) ([]models.Snapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, market_id, yes_prob, no_prob, timestamp, source
		FROM snapshots WHERE market_id = ? ORDER BY timestamp ASC`, marketID)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

func (s *Storage) GetSnapshotsInWindow(marketID string, window time.Duration) ([]models.Snapshot, error) {
	cutoff := time.Now().Add(-window).UnixNano()
	rows, err := s.db.Query(`
		SELECT id, market_id, yes_prob, no_prob, timestamp, source
		FROM snapshots WHERE market_id = ? AND timestamp >= ? ORDER BY timestamp ASC`,
		marketID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots in window: %w", err)
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

// --- Changes ---

func (s *Storage) AddChange(change *models.Change) error {
	if err := change.Validate(); err != nil {
		return fmt.Errorf("invalid change: %w", err)
	}
	_, err := s.db.Exec(`
		INSERT INTO changes
			(id, market_id, original_event_id, event_title, event_url, polymarket_market_id,
			 market_question, magnitude, direction, old_prob, new_prob, time_window,
			 detected_at, notified, signal_score)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		change.ID, change.EventID, change.OriginalEventID, change.EventTitle, change.EventURL,
		change.MarketID, change.MarketQuestion,
		change.Magnitude, change.Direction, change.OldProbability, change.NewProbability,
		change.TimeWindow.Nanoseconds(), change.DetectedAt.UnixNano(),
		boolToInt(change.Notified), change.SignalScore,
	)
	if err != nil {
		return fmt.Errorf("failed to insert change: %w", err)
	}
	return nil
}

func (s *Storage) GetTopChanges(k int) ([]models.Change, error) {
	rows, err := s.db.Query(`
		SELECT id, market_id, original_event_id, event_title, event_url, polymarket_market_id,
		       market_question, magnitude, direction, old_prob, new_prob, time_window,
		       detected_at, notified, signal_score
		FROM changes ORDER BY magnitude DESC LIMIT ?`, k)
	if err != nil {
		return nil, fmt.Errorf("failed to query changes: %w", err)
	}
	defer rows.Close()
	return scanChanges(rows)
}

func (s *Storage) ClearChanges() error {
	if _, err := s.db.Exec(`DELETE FROM changes`); err != nil {
		return fmt.Errorf("failed to clear changes: %w", err)
	}
	return nil
}

// --- Rotation ---

// RotateSnapshots keeps at most maxSnapshotsPerEvent newest snapshots per market,
// ordered by timestamp (not insertion order).
func (s *Storage) RotateSnapshots() error {
	rows, err := s.db.Query(`
		SELECT market_id FROM snapshots
		GROUP BY market_id HAVING COUNT(*) > ?`, s.maxSnapshotsPerEvent)
	if err != nil {
		return fmt.Errorf("failed to query markets for rotation: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		_, err := s.db.Exec(`
			DELETE FROM snapshots
			WHERE market_id = ? AND id NOT IN (
				SELECT id FROM snapshots WHERE market_id = ?
				ORDER BY timestamp DESC LIMIT ?
			)`, id, id, s.maxSnapshotsPerEvent)
		if err != nil {
			return fmt.Errorf("failed to rotate snapshots for %s: %w", id, err)
		}
	}
	return nil
}

// RotateMarkets keeps at most maxMarkets newest markets (by last_updated),
// cascading delete removes their snapshots.
func (s *Storage) RotateMarkets() error {
	_, err := s.db.Exec(`
		DELETE FROM markets WHERE id NOT IN (
			SELECT id FROM markets ORDER BY last_updated DESC LIMIT ?
		)`, s.maxMarkets)
	if err != nil {
		return fmt.Errorf("failed to rotate markets: %w", err)
	}
	return nil
}

// --- Helpers ---

const marketCols = `id, event_id, market_id, market_question, title, event_url, description,
	category, subcategory, yes_prob, no_prob, volume_24hr, volume_1wk, volume_1mo,
	liquidity, active, closed, last_updated, created_at`

func scanMarket(scan func(...any) error) (*models.Market, error) {
	var m models.Market
	var lastUpdatedNano, createdAtNano int64
	var active, closed int
	err := scan(
		&m.ID, &m.EventID, &m.MarketID, &m.MarketQuestion, &m.Title, &m.EventURL,
		&m.Description, &m.Category, &m.Subcategory,
		&m.YesProbability, &m.NoProbability,
		&m.Volume24hr, &m.Volume1wk, &m.Volume1mo, &m.Liquidity,
		&active, &closed, &lastUpdatedNano, &createdAtNano,
	)
	if err != nil {
		return nil, err
	}
	m.Active = active != 0
	m.Closed = closed != 0
	m.LastUpdated = time.Unix(0, lastUpdatedNano)
	m.CreatedAt = time.Unix(0, createdAtNano)
	return &m, nil
}

func scanSnapshots(rows *sql.Rows) ([]models.Snapshot, error) {
	var result []models.Snapshot
	for rows.Next() {
		var s models.Snapshot
		var tsNano int64
		if err := rows.Scan(&s.ID, &s.EventID, &s.YesProbability, &s.NoProbability, &tsNano, &s.Source); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}
		s.Timestamp = time.Unix(0, tsNano)
		result = append(result, s)
	}
	if result == nil {
		result = []models.Snapshot{}
	}
	return result, rows.Err()
}

func scanChanges(rows *sql.Rows) ([]models.Change, error) {
	var result []models.Change
	for rows.Next() {
		var c models.Change
		var detectedAtNano, timeWindowNano int64
		var notified int
		err := rows.Scan(
			&c.ID, &c.EventID, &c.OriginalEventID, &c.EventTitle, &c.EventURL,
			&c.MarketID, &c.MarketQuestion,
			&c.Magnitude, &c.Direction, &c.OldProbability, &c.NewProbability,
			&timeWindowNano, &detectedAtNano, &notified, &c.SignalScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan change: %w", err)
		}
		c.TimeWindow = time.Duration(timeWindowNano)
		c.DetectedAt = time.Unix(0, detectedAtNano)
		c.Notified = notified != 0
		result = append(result, c)
	}
	return result, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
