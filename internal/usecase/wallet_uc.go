package usecase

import (
	"context"
	"high-perf-wallet/internal/domain"
)

type walletUsecase struct {
	repo domain.WalletRepository
}

func NewWalletUsecase(repo domain.WalletRepository) domain.WalletUsecase {
	return &walletUsecase{repo: repo}
}

func (u *walletUsecase) GetByID(ctx context.Context, id int64) (*domain.Account, error) {
	return u.repo.GetByID(ctx, id)
}
