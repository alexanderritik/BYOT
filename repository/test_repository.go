package repository

import (
	"context"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestRepository interface {
	Create(ctx context.Context, test *model.Test) error
	GetByID(ctx context.Context, uuid string) (*model.Test, error)
}

type postgresTestRepository struct {
	pool *pgxpool.Pool
}

func NewTestRepository(pool *pgxpool.Pool) TestRepository {
	return &postgresTestRepository{pool: pool}
}

func (r *postgresTestRepository) Create(ctx context.Context, test *model.Test) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tests (uuid, runtime, original_filename, severity, binary_url)
         VALUES ($1, $2, $3, $4, $5)`,
		test.UUID,
		test.Runtime,
		test.OriginalFilename,
		test.Severity,
		test.BinaryURL,
	)
	return err
}

func (r *postgresTestRepository) GetByID(ctx context.Context, uuid string) (*model.Test, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid, runtime, original_filename, severity, binary_url, created_at
         FROM tests WHERE uuid = $1`, uuid,
	)
	if err != nil {
		return nil, err
	}
	test, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Test])
	if err != nil {
		return nil, err
	}
	return &test, nil
}
