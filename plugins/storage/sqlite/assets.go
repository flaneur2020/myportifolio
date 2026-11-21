package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"portifolio/core"
)

func (s *Store) assetSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS assets(market TEXT NOT NULL,symbol TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL,PRIMARY KEY(market,symbol)); CREATE TABLE IF NOT EXISTS asset_directory(market TEXT PRIMARY KEY,updated_at TEXT NOT NULL);`)
	return err
}

func (s *Store) Assets(ctx context.Context, market string) ([]core.Asset, time.Time, error) {
	if err := s.assetSchema(ctx); err != nil {
		return nil, time.Time{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT symbol,name,kind FROM assets WHERE market=? ORDER BY symbol`, market)
	if err != nil {
		return nil, time.Time{}, err
	}
	var out []core.Asset
	for rows.Next() {
		a := core.Asset{Market: market}
		if err = rows.Scan(&a.Symbol, &a.Name, &a.Kind); err != nil {
			rows.Close()
			return nil, time.Time{}, err
		}
		out = append(out, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, time.Time{}, err
	}
	var stamp string
	_ = s.db.QueryRowContext(ctx, `SELECT updated_at FROM asset_directory WHERE market=?`, market).Scan(&stamp)
	updated, _ := time.Parse(time.RFC3339, stamp)
	return out, updated, nil
}

func (s *Store) PutAssets(ctx context.Context, market string, assets []core.Asset) error {
	if len(assets) == 0 {
		return fmt.Errorf("refusing to replace directory with empty list")
	}
	for _, a := range assets {
		if a.Market != market || strings.TrimSpace(a.Symbol) == "" || strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Kind) == "" {
			return fmt.Errorf("invalid directory asset %q", a.Symbol)
		}
	}
	if err := s.assetSchema(ctx); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM assets WHERE market=?`, market); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO assets(market,symbol,name,kind) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, a := range assets {
		if _, err = stmt.ExecContext(ctx, a.Market, a.Symbol, a.Name, a.Kind); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO asset_directory(market,updated_at) VALUES(?,?) ON CONFLICT(market) DO UPDATE SET updated_at=excluded.updated_at`, market, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	return tx.Commit()
}
