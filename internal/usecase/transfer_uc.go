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
	repo            domain.WalletRepository
	fxServ          domain.ExchangeRateService
	auditDispatcher domain.AuditDispatcher
}

func NewTransferUsecase(
	repo domain.WalletRepository,
	fxServ domain.ExchangeRateService,
	auditDispatcher domain.AuditDispatcher,
) domain.TransferUsecase {
	return &transferUsecase{
		repo:            repo,
		fxServ:          fxServ,
		auditDispatcher: auditDispatcher,
	}
}

func (u *transferUsecase) ExecuteTransfer(ctx context.Context, idempotencyKey string, fromID, toID int64, amount int64) (*domain.Transfer, error) {
	if amount <= 0 {
		return nil, apperror.NewValidationError("INVALID_AMOUNT", "Transfer amount must be greater than zero")
	}
	if fromID == toID {
		return nil, apperror.NewValidationError("CANNOT_TRANSFER_TO_SELF", "Sender and recipient accounts cannot be the same")
	}

	// 1. Ambil info akun non-locking terlebih dahulu untuk mengetahui mata uang.
	// Ini dilakukan sebelum memulai transaksi DB agar pemanggilan external FX API (jika cache miss)
	// tidak menahan row-level lock terlalu lama yang bisa mengakibatkan DB bottleneck/deadlock.
	fromAccInfo, err := u.repo.GetByID(ctx, fromID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", fromID))
		}
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to retrieve sender account info", err)
	}

	toAccInfo, err := u.repo.GetByID(ctx, toID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", toID))
		}
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to retrieve recipient account info", err)
	}

	// 2. Ambil Exchange Rate
	rate, err := u.fxServ.GetRate(ctx, fromAccInfo.Currency, toAccInfo.Currency)
	if err != nil {
		return nil, apperror.NewInternalError("FX_SERVICE_ERROR", "Failed to retrieve currency exchange rate", err)
	}

	targetAmount := amount
	if fromAccInfo.Currency != toAccInfo.Currency {
		targetAmount = int64(float64(amount) * rate)
	}

	// 3. Memulai Transaksi Database
	tx, err := u.repo.BeginTx(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("Gagal memulai transaksi", zap.Error(err))
		return nil, apperror.NewInternalError("TRANSACTION_ERROR", "Failed to start database transaction", err)
	}

	// Pastikan rollback dijalankan jika fungsi keluar sebelum commit
	defer func() {
		_ = u.repo.RollbackTx(ctx, tx)
	}()

	// 4. Mencegah Deadlock dengan mengunci ID terkecil terlebih dahulu (Standard Aturan DB)
	firstID, secondID := fromID, toID
	if fromID > toID {
		firstID, secondID = toID, fromID
	}

	// Kunci Row Pertama
	acc1, err := u.repo.GetByIDForUpdate(ctx, tx, firstID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", firstID))
		}
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to lock account row in database", err)
	}
	// Kunci Row Kedua
	acc2, err := u.repo.GetByIDForUpdate(ctx, tx, secondID)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil, apperror.NewNotFoundError("ACCOUNT_NOT_FOUND", fmt.Sprintf("Account with ID %d was not found", secondID))
		}
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to lock account row in database", err)
	}

	var fromAcc, toAcc *domain.Account
	if fromID == firstID {
		fromAcc = acc1
		toAcc = acc2
	} else {
		fromAcc = acc2
		toAcc = acc1
	}

	// 5. Validasi saldo pengirim (menggunakan saldo ter-lock terbaru)
	if fromAcc.Balance < amount {
		return nil, apperror.NewValidationError("INSUFFICIENT_FUNDS", fmt.Sprintf("Account %d has insufficient funds", fromID))
	}

	// 6. Eksekusi Mutasi Saldo
	err = u.repo.UpdateBalance(ctx, tx, fromID, fromAcc.Balance-amount)
	if err != nil {
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to update sender account balance", err)
	}

	err = u.repo.UpdateBalance(ctx, tx, toID, toAcc.Balance+targetAmount)
	if err != nil {
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to update recipient account balance", err)
	}

	// 7. Catat Histori Transfer
	transfer := &domain.Transfer{
		FromAccountID:  fromID,
		ToAccountID:    toID,
		SourceCurrency: fromAcc.Currency,
		TargetCurrency: toAcc.Currency,
		SourceAmount:   amount,
		TargetAmount:   targetAmount,
		ExchangeRate:   rate,
	}
	err = u.repo.CreateTransfer(ctx, tx, transfer)
	if err != nil {
		return nil, apperror.NewInternalError("DATABASE_ERROR", "Failed to record transaction history", err)
	}

	// 8. Commit Transaksi jika semua sukses
	if err := u.repo.CommitTx(ctx, tx); err != nil {
		return nil, apperror.NewInternalError("TRANSACTION_ERROR", "Failed to commit database transaction", err)
	}

	logger.WithCtx(ctx).Info("Transfer Sukses",
		zap.Int64("from", fromID),
		zap.Int64("to", toID),
		zap.String("from_currency", fromAcc.Currency),
		zap.String("to_currency", toAcc.Currency),
		zap.Int64("source_amount", amount),
		zap.Int64("target_amount", targetAmount),
		zap.Float64("exchange_rate", rate),
	)

	// Dispatch event secara asinkron ke background audit worker
	u.auditDispatcher.Dispatch(transfer)

	return transfer, nil
}
