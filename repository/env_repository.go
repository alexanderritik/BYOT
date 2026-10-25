package repository

import (
	"context"
	"errors"

	"github.com/alexanderritik/byot/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTooManyEnvVars = errors.New("too many environment variables")

type EnvRepository interface {
	List(ctx context.Context, testID string) ([]model.EnvVar, error)
	// Upsert inserts or replaces a variable, enforcing maxVars per test.
	Upsert(ctx context.Context, testID string, v model.EnvVar, maxVars int) error
	Delete(ctx context.Context, testID, name string) error
}

type postgresEnvRepository struct {
	pool *pgxpool.Pool
}

func NewEnvRepository(pool *pgxpool.Pool) EnvRepository {
	return &postgresEnvRepository{pool: pool}
}

func (r *postgresEnvRepository) List(ctx context.Context, testID string) ([]model.EnvVar, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT name, value, is_secret, updated_at FROM test_env_vars WHERE test_id = $1 ORDER BY name`, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.EnvVar{}
	for rows.Next() {
		var v model.EnvVar
		if err := rows.Scan(&v.Name, &v.Value, &v.IsSecret, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *postgresEnvRepository) Upsert(ctx context.Context, testID string, v model.EnvVar, maxVars int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Serialise writers per test so the count check can't be raced past.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM tests WHERE uuid = $1 FOR UPDATE`, testID); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM test_env_vars WHERE test_id = $1 AND name = $2)`, testID, v.Name).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM test_env_vars WHERE test_id = $1`, testID).Scan(&n); err != nil {
			return err
		}
		if n >= maxVars {
			return ErrTooManyEnvVars
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO test_env_vars (test_id, name, value, is_secret, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (test_id, name)
		DO UPDATE SET value = EXCLUDED.value, is_secret = EXCLUDED.is_secret, updated_at = NOW()`,
		testID, v.Name, v.Value, v.IsSecret); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *postgresEnvRepository) Delete(ctx context.Context, testID, name string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM test_env_vars WHERE test_id = $1 AND name = $2`, testID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
