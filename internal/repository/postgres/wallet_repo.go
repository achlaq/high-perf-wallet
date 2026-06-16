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
	query := "SELECT id, name, balance, currency FROM accounts WHERE id = $1 FOR UPDATE"

	currentTx := tx.(pgx.Tx)
	acc := &domain.Account{}

	err := currentTx.QueryRow(ctx, query, id).Scan(&acc.ID, &acc.Name, &acc.Balance, &acc.Currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
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
	query := "SELECT id, name, balance, currency FROM accounts WHERE id = $1"
	acc := &domain.Account{}

	err := r.db.QueryRow(ctx, query, id).Scan(&acc.ID, &acc.Name, &acc.Balance, &acc.Currency)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAccountNotFound
		}
		return nil, err
	}
	return acc, nil
}

func (r *walletRepository) CreateTransfer(ctx context.Context, tx any, transfer *domain.Transfer) error {
	query := `INSERT INTO transfers (from_account_id, to_account_id, source_currency, target_currency, source_amount, target_amount, exchange_rate) 
	          VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at`
	currentTx := tx.(pgx.Tx)

	err := currentTx.QueryRow(ctx, query, 
		transfer.FromAccountID, 
		transfer.ToAccountID, 
		transfer.SourceCurrency, 
		transfer.TargetCurrency, 
		transfer.SourceAmount, 
		transfer.TargetAmount, 
		transfer.ExchangeRate,
	).Scan(&transfer.ID, &transfer.CreatedAt)
	return err
}

func (r *walletRepository) GetTransfersByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]*domain.Transfer, int64, error) {
	// 1. Get total count of transfers for this account
	countQuery := "SELECT COUNT(*) FROM transfers WHERE from_account_id = $1 OR to_account_id = $1"
	var totalCount int64
	err := r.db.QueryRow(ctx, countQuery, accountID).Scan(&totalCount)
	if err != nil {
		return nil, 0, err
	}

	// 2. Get paginated transfers list
	query := `SELECT id, from_account_id, to_account_id, source_currency, target_currency, source_amount, target_amount, exchange_rate, created_at 
	          FROM transfers 
	          WHERE from_account_id = $1 OR to_account_id = $1 
	          ORDER BY created_at DESC 
	          LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, accountID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transfers []*domain.Transfer
	for rows.Next() {
		t := &domain.Transfer{}
		err := rows.Scan(
			&t.ID, 
			&t.FromAccountID, 
			&t.ToAccountID, 
			&t.SourceCurrency, 
			&t.TargetCurrency, 
			&t.SourceAmount, 
			&t.TargetAmount, 
			&t.ExchangeRate, 
			&t.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		transfers = append(transfers, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return transfers, totalCount, nil
}
