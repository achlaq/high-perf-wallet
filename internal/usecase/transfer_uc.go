package usecase

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"high-perf-wallet/internal/domain"
	"high-perf-wallet/pkg/apperror"
	"high-perf-wallet/pkg/logger"
)

type transferUsecase struct {
	repo domain.WalletRepository
}

func NewTransferUsecase(repo domain.WalletRepository) domain.TransferUsecase {
	return &transferUsecase{repo: repo}
}

func (u *transferUsecase) ExecuteTransfer(ctx context.Context, idempotencyKey string, fromID, toID int64, amount int64) error {
	if amount <= 0 {
		return apperror.NewValidationError("INVALID_AMOUNT", "Transfer amount must be greater than zero")
	}
	if fromID == toID {
		return apperror.NewValidationError("CANNOT_TRANSFER_TO_SELF", "Sender and recipient accounts cannot be the same")
	}

	// 1. Memulai Transaksi Database
	tx, err := u.repo.BeginTx(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("Gagal memulai transaksi", zap.Error(err))
		return apperror.NewInternalError("TRANSACTION_ERROR", "Failed to start database transaction", err)
	}

	// Pastikan rollback dijalankan jika fungsi keluar sebelum commit
	defer func() {
		_ = u.repo.RollbackTx(ctx, tx)
	}()

	// 2. Mencegah Deadlock dengan mengunci ID terkecil terlebih dahulu (Standard Aturan DB)
	firstID, secondID := fromID, toID
	if fromID > toID {
		firstID, secondID = toID, fromID
	}

	// Kunci Row Pertama
	acc1, err := u.repo.GetByIDForUpdate(ctx, tx, firstID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", firstID))
		}
		return apperror.NewInternalError("DATABASE_ERROR", "Failed to lock account row in database", err)
	}
	// Kunci Row Kedua
	acc2, err := u.repo.GetByIDForUpdate(ctx, tx, secondID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", secondID))
		}
		return apperror.NewInternalError("DATABASE_ERROR", "Failed to lock account row in database", err)
	}

	var fromAcc, toAcc *domain.Account
	if fromID == firstID {
		fromAcc = acc1
		toAcc = acc2
	} else {
		fromAcc = acc2
		toAcc = acc1
	}

	// 3. Validasi saldo pengirim
	if fromAcc.Balance < amount {
		return apperror.NewValidationError("INSUFFICIENT_FUNDS", fmt.Sprintf("Account %d has insufficient funds", fromID))
	}

	// 4. Eksekusi Mutasi Saldo
	err = u.repo.UpdateBalance(ctx, tx, fromID, fromAcc.Balance-amount)
	if err != nil {
		return apperror.NewInternalError("DATABASE_ERROR", "Failed to update sender account balance", err)
	}

	err = u.repo.UpdateBalance(ctx, tx, toID, toAcc.Balance+amount)
	if err != nil {
		return apperror.NewInternalError("DATABASE_ERROR", "Failed to update recipient account balance", err)
	}

	// 4.5 Catat Histori Transfer
	transfer := &domain.Transfer{
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        amount,
	}
	err = u.repo.CreateTransfer(ctx, tx, transfer)
	if err != nil {
		return apperror.NewInternalError("DATABASE_ERROR", "Failed to record transaction history", err)
	}

	// 5. Commit Transaksi jika semua sukses
	if err := u.repo.CommitTx(ctx, tx); err != nil {
		return apperror.NewInternalError("TRANSACTION_ERROR", "Failed to commit database transaction", err)
	}

	logger.WithCtx(ctx).Info("Transfer Sukses",
		zap.Int64("from", fromID),
		zap.Int64("to", toID),
		zap.Int64("amount", amount),
	)
	return nil
}
