package usecase

import (
	"context"
	"errors"
	"fmt"
	"high-perf-wallet/internal/domain"
	"high-perf-wallet/pkg/apperror"
)

type walletUsecase struct {
	repo domain.WalletRepository
}

func NewWalletUsecase(repo domain.WalletRepository) domain.WalletUsecase {
	return &walletUsecase{repo: repo}
}

func (u *walletUsecase) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	acc, err := u.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", id))
		}
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to retrieve account details", err)
	}
	return acc, nil
}

func (u *walletUsecase) GetTransfers(ctx context.Context, accountID int64, limit, offset int) (*domain.PaginatedTransfers, error) {
	transfers, totalCount, err := u.repo.GetTransfersByAccountID(ctx, accountID, limit, offset)
	if err != nil {
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to retrieve transfer history", err)
	}

	return &domain.PaginatedTransfers{
		Transfers:  transfers,
		TotalCount: totalCount,
		Limit:      limit,
		Offset:     offset,
	}, nil
}
