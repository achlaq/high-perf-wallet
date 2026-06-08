package domain

import "context"

type Account struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Balance int64  `json:"balance"`
}

type WalletRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	// Ambil data dengan Row-Level Locking untuk mencegah data balapan
	GetByIDForUpdate(ctx context.Context, tx any, id int64) (*Account, error)
	UpdateBalance(ctx context.Context, tx any, id int64, newBalance int64) error

	// Tx Manager helper
	BeginTx(ctx context.Context) (any, error)
	CommitTx(ctx context.Context, tx any) error
	RollbackTx(ctx context.Context, tx any) error
}

type WalletUsecase interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type TransferUsecase interface {
	ExecuteTransfer(ctx context.Context, idempotencyKey string, fromID, toID int64, amount int64) error
}
