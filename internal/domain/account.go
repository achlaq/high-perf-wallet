package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrAccountNotFound = errors.New("account_not_found")
)

type Account struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Balance int64  `json:"balance"`
}

type Transfer struct {
	ID            int64     `json:"id"`
	FromAccountID int64     `json:"from_account_id"`
	ToAccountID   int64     `json:"to_account_id"`
	Amount        int64     `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
}

type PaginatedTransfers struct {
	Transfers  []*Transfer `json:"transfers"`
	TotalCount int64       `json:"total_count"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
}

type WalletRepository interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	// Ambil data dengan Row-Level Locking untuk mencegah data balapan
	GetByIDForUpdate(ctx context.Context, tx any, id int64) (*Account, error)
	UpdateBalance(ctx context.Context, tx any, id int64, newBalance int64) error
	CreateTransfer(ctx context.Context, tx any, transfer *Transfer) error
	GetTransfersByAccountID(ctx context.Context, accountID int64, limit, offset int) ([]*Transfer, int64, error)

	// Tx Manager helper
	BeginTx(ctx context.Context) (any, error)
	CommitTx(ctx context.Context, tx any) error
	RollbackTx(ctx context.Context, tx any) error
}

type WalletUsecase interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
	GetTransfers(ctx context.Context, accountID int64, limit, offset int) (*PaginatedTransfers, error)
}

type TransferUsecase interface {
	ExecuteTransfer(ctx context.Context, idempotencyKey string, fromID, toID int64, amount int64) error
}

type Idempotency struct {
	Status       string `json:"status"`
	ResponseCode int    `json:"response_code"`
	ResponseBody string `json:"response_body"`
}

type IdempotencyRepository interface {
	Get(ctx context.Context, key string) (*Idempotency, error)
	Set(ctx context.Context, key string, val *Idempotency, ttl time.Duration) error
}
