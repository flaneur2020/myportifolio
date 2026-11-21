package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
	"portifolio/core"
)

// Store is the SQLite storage plugin implementing core.PriceStore and core.DirectoryStore.
type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	_, err = db.ExecContext(ctx, `PRAGMA journal_mode=WAL; CREATE TABLE IF NOT EXISTS prices(market TEXT NOT NULL,symbol TEXT NOT NULL,date TEXT NOT NULL,open REAL,high REAL,low REAL,close REAL,adjusted_close REAL,currency TEXT,provider TEXT,updated_at TEXT NOT NULL,PRIMARY KEY(market,symbol,date)); CREATE INDEX IF NOT EXISTS prices_range ON prices(market,symbol,date);`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Put(ctx context.Context, p core.Position, bars []core.Bar) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO prices(market,symbol,date,open,high,low,close,adjusted_close,currency,provider,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(market,symbol,date) DO UPDATE SET open=excluded.open,high=excluded.high,low=excluded.low,close=excluded.close,adjusted_close=excluded.adjusted_close,currency=excluded.currency,provider=excluded.provider,updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, b := range bars {
		_, err = stmt.ExecContext(ctx, p.Market, p.Symbol, b.Date.Format("2006-01-02"), b.Open, b.High, b.Low, b.Close, b.AdjustedClose, b.Currency, b.Source, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) History(ctx context.Context, p core.Position, start, end time.Time) ([]core.Bar, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date,open,high,low,close,adjusted_close,currency,provider FROM prices WHERE market=? AND symbol=? AND date>=? AND date<=? ORDER BY date`, p.Market, p.Symbol, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Bar
	for rows.Next() {
		var date, currency, source string
		var b core.Bar
		if err = rows.Scan(&date, &b.Open, &b.High, &b.Low, &b.Close, &b.AdjustedClose, &currency, &source); err != nil {
			return nil, err
		}
		b.Date, err = time.Parse("2006-01-02", date)
		if err != nil {
			return nil, err
		}
		b.Currency = currency
		b.Source = source
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) Health() error {
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM prices").Scan(&n); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	return nil
}
