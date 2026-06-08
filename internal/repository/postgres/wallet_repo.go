package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"high-perf-wallet/internal/domain"
)

type walletRepository struct {
	db *pgxpool.Pool
}

func NewWalletRepository(db *pgxpool.Pool) domain.WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) BeginTx(ctx context.Context) (any, error) {
	return r.db.Begin(ctx)
}

func (r *walletRepository) CommitTx(ctx context.Context, tx any) error {
	return tx.(pgx.Tx).Commit(ctx)
}

func (r *walletRepository) RollbackTx(ctx context.Context, tx any) error {
	return tx.(pgx.Tx).Rollback(ctx)
}

func (r *walletRepository) GetByIDForUpdate(ctx context.Context, tx any, id int64) (*domain.Account, error) {
	query := "SELECT id, name, balance FROM accounts WHERE id = $1 FOR UPDATE"

	currentTx := tx.(pgx.Tx)
	acc := &domain.Account{}

	err := currentTx.QueryRow(ctx, query, id).Scan(&acc.ID, &acc.Name, &acc.Balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("account_not_found")
		}
		return nil, err
	}
	return acc, nil
}

func (r *walletRepository) UpdateBalance(ctx context.Context, tx any, id int64, newBalance int64) error {
	query := "UPDATE accounts SET balance = $1 WHERE id = $2"
	currentTx := tx.(pgx.Tx)

	_, err := currentTx.Exec(ctx, query, newBalance, id)
	return err
}

func (r *walletRepository) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	query := "SELECT id, name, balance FROM accounts WHERE id = $1"
	acc := &domain.Account{}

	err := r.db.QueryRow(ctx, query, id).Scan(&acc.ID, &acc.Name, &acc.Balance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("account_not_found")
		}
		return nil, err
	}
	return acc, nil
}
