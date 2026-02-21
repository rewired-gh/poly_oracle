// Package storage provides SQLite-backed persistence for markets, states, and alerts.
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rewired-gh/polyoracle/internal/models"
	_ "modernc.org/sqlite"
)

// Storage wraps a SQLite database for all persistence operations.
type Storage struct {
	db         *sql.DB
	maxMarkets int
}

// New opens or creates the SQLite database at dbPath.
// An empty dbPath defaults to $TMPDIR/polyoracle/data.db.
func New(maxMarkets int, dbPath string) (*Storage, error) {
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
	db.SetMaxOpenConns(1) // single writer; WAL allows concurrent readers
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	s := &Storage{db: db, maxMarkets: maxMarkets}
	if err := s.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS markets (
			id              TEXT PRIMARY KEY,
			event_id        TEXT NOT NULL,
			market_id       TEXT NOT NULL,
			market_question TEXT,
			title           TEXT NOT NULL,
			event_url       TEXT,
			category        TEXT NOT NULL,
			yes_prob        REAL NOT NULL,
			volume_24hr     REAL,
			liquidity       REAL,
			last_updated    INTEGER NOT NULL,
			created_at      INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS market_state (
			market_id       TEXT PRIMARY KEY REFERENCES markets(id) ON DELETE CASCADE,
			welford_count   INTEGER NOT NULL DEFAULT 0,
			welford_mean    REAL NOT NULL DEFAULT 0,
			welford_m2      REAL NOT NULL DEFAULT 0,
			avg_depth       REAL NOT NULL DEFAULT 0,
			tc_buffer       TEXT NOT NULL DEFAULT '[]',
			tc_index        INTEGER NOT NULL DEFAULT 0,
			last_price      REAL NOT NULL DEFAULT 0,
			last_sigma      REAL NOT NULL DEFAULT 0.01,
			last_volume     REAL NOT NULL DEFAULT 0,
			updated_at      INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id              TEXT PRIMARY KEY,
			market_id       TEXT NOT NULL REFERENCES markets(id) ON DELETE CASCADE,
			event_title     TEXT NOT NULL,
			event_url       TEXT,
			market_question TEXT,
			final_score     REAL NOT NULL,
			hellinger_dist  REAL NOT NULL,
			liq_pressure    REAL NOT NULL,
			inst_energy     REAL NOT NULL,
			tc              REAL NOT NULL,
			old_prob        REAL NOT NULL,
			new_prob        REAL NOT NULL,
			price_delta     REAL NOT NULL,
			volume_delta    REAL NOT NULL,
			depth           REAL NOT NULL,
			detected_at     INTEGER NOT NULL,
			notified        INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_detected_at ON alerts(detected_at)`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_score ON alerts(final_score DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

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
			(id, event_id, market_id, market_question, title, event_url, category,
			 yes_prob, volume_24hr, liquidity, last_updated, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		market.ID, market.EventID, market.MarketID, market.MarketQuestion, market.Title,
		market.EventURL, market.Category,
		market.YesProbability, market.Volume24hr, market.Liquidity,
		market.LastUpdated.UnixNano(), market.CreatedAt.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert market: %w", err)
	}

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
			event_id=?, market_id=?, market_question=?, title=?, event_url=?, category=?,
			yes_prob=?, volume_24hr=?, liquidity=?, last_updated=?, created_at=?
		WHERE id=?`,
		market.EventID, market.MarketID, market.MarketQuestion, market.Title,
		market.EventURL, market.Category,
		market.YesProbability, market.Volume24hr, market.Liquidity,
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

func (s *Storage) SaveState(marketID string, state *models.MarketState) error {
	tcBufferJSON, err := json.Marshal(state.TCBuffer)
	if err != nil {
		return fmt.Errorf("failed to marshal TC buffer: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO market_state
			(market_id, welford_count, welford_mean, welford_m2, avg_depth,
			 tc_buffer, tc_index, last_price, last_sigma, last_volume, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		marketID, state.WelfordCount, state.WelfordMean, state.WelfordM2, state.AvgDepth,
		string(tcBufferJSON), state.TCIndex, state.LastPrice, state.LastSigma, state.LastVolume,
		state.UpdatedAt.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	return nil
}

func (s *Storage) LoadState(marketID string) (*models.MarketState, error) {
	row := s.db.QueryRow(`
		SELECT market_id, welford_count, welford_mean, welford_m2, avg_depth,
		       tc_buffer, tc_index, last_price, last_sigma, last_volume, updated_at
		FROM market_state WHERE market_id = ?`, marketID)

	var state models.MarketState
	var tcBufferJSON string
	var updatedAtNano int64

	err := row.Scan(
		&state.MarketID, &state.WelfordCount, &state.WelfordMean, &state.WelfordM2, &state.AvgDepth,
		&tcBufferJSON, &state.TCIndex, &state.LastPrice, &state.LastSigma, &state.LastVolume,
		&updatedAtNano,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	if err := json.Unmarshal([]byte(tcBufferJSON), &state.TCBuffer); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TC buffer: %w", err)
	}

	state.UpdatedAt = time.Unix(0, updatedAtNano)
	return &state, nil
}

func (s *Storage) LoadAllStates() (map[string]*models.MarketState, error) {
	rows, err := s.db.Query(`
		SELECT market_id, welford_count, welford_mean, welford_m2, avg_depth,
		       tc_buffer, tc_index, last_price, last_sigma, last_volume, updated_at
		FROM market_state`)
	if err != nil {
		return nil, fmt.Errorf("failed to query states: %w", err)
	}
	defer rows.Close()

	states := make(map[string]*models.MarketState)
	for rows.Next() {
		var state models.MarketState
		var tcBufferJSON string
		var updatedAtNano int64

		err := rows.Scan(
			&state.MarketID, &state.WelfordCount, &state.WelfordMean, &state.WelfordM2, &state.AvgDepth,
			&tcBufferJSON, &state.TCIndex, &state.LastPrice, &state.LastSigma, &state.LastVolume,
			&updatedAtNano,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan state: %w", err)
		}

		if err := json.Unmarshal([]byte(tcBufferJSON), &state.TCBuffer); err != nil {
			return nil, fmt.Errorf("failed to unmarshal TC buffer: %w", err)
		}

		state.UpdatedAt = time.Unix(0, updatedAtNano)
		states[state.MarketID] = &state
	}

	return states, rows.Err()
}

func (s *Storage) AddAlert(alert *models.Alert) error {
	_, err := s.db.Exec(`
		INSERT INTO alerts
			(id, market_id, event_title, event_url, market_question, final_score,
			 hellinger_dist, liq_pressure, inst_energy, tc, old_prob, new_prob,
			 price_delta, volume_delta, depth, detected_at, notified)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		alert.MarketID, alert.MarketID, alert.EventTitle, alert.EventURL, alert.MarketQuestion,
		alert.FinalScore, alert.HellingerDist, alert.LiqPressure, alert.InstEnergy, alert.TC,
		alert.OldProb, alert.NewProb, alert.PriceDelta, alert.VolumeDelta, alert.Depth,
		alert.DetectedAt.UnixNano(), boolToInt(alert.Notified),
	)
	if err != nil {
		return fmt.Errorf("failed to insert alert: %w", err)
	}
	return nil
}

func (s *Storage) GetTopAlerts(k int) ([]models.Alert, error) {
	rows, err := s.db.Query(`
		SELECT id, market_id, event_title, event_url, market_question, final_score,
		       hellinger_dist, liq_pressure, inst_energy, tc, old_prob, new_prob,
		       price_delta, volume_delta, depth, detected_at, notified
		FROM alerts ORDER BY final_score DESC LIMIT ?`, k)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer rows.Close()

	var alerts []models.Alert
	for rows.Next() {
		var a models.Alert
		var detectedAtNano int64
		var notified int

		err := rows.Scan(
			&a.MarketID, &a.MarketID, &a.EventTitle, &a.EventURL, &a.MarketQuestion,
			&a.FinalScore, &a.HellingerDist, &a.LiqPressure, &a.InstEnergy, &a.TC,
			&a.OldProb, &a.NewProb, &a.PriceDelta, &a.VolumeDelta, &a.Depth,
			&detectedAtNano, &notified,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan alert: %w", err)
		}

		a.DetectedAt = time.Unix(0, detectedAtNano)
		a.Notified = notified != 0
		alerts = append(alerts, a)
	}

	return alerts, rows.Err()
}

func (s *Storage) ClearAlerts() error {
	if _, err := s.db.Exec(`DELETE FROM alerts`); err != nil {
		return fmt.Errorf("failed to clear alerts: %w", err)
	}
	return nil
}

// RotateMarkets keeps at most maxMarkets newest markets by last_updated.
// Cascading deletes remove associated state and alerts.
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

const marketCols = `id, event_id, market_id, market_question, title, event_url, category,
	yes_prob, volume_24hr, liquidity, last_updated, created_at`

func scanMarket(scan func(...any) error) (*models.Market, error) {
	var m models.Market
	var lastUpdatedNano, createdAtNano int64
	err := scan(
		&m.ID, &m.EventID, &m.MarketID, &m.MarketQuestion, &m.Title, &m.EventURL,
		&m.Category,
		&m.YesProbability, &m.Volume24hr, &m.Liquidity,
		&lastUpdatedNano, &createdAtNano,
	)
	if err != nil {
		return nil, err
	}
	m.Active = true
	m.Closed = false
	m.LastUpdated = time.Unix(0, lastUpdatedNano)
	m.CreatedAt = time.Unix(0, createdAtNano)
	return &m, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
