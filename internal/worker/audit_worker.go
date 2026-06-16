package worker

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"high-perf-wallet/internal/domain"
	"high-perf-wallet/pkg/logger"
	"time"
)

type auditWorker struct {
	eventChan chan *domain.Transfer
}

func NewAuditWorker(bufferSize int) *auditWorker {
	return &auditWorker{
		eventChan: make(chan *domain.Transfer, bufferSize),
	}
}

// Dispatch sends a transfer event to the channel. It implements domain.AuditDispatcher.
func (w *auditWorker) Dispatch(transfer *domain.Transfer) {
	select {
	case w.eventChan <- transfer:
		// Event sent successfully to buffer
	default:
		// Channel buffer is full, log a warning to prevent blocking the main thread (fail-safe)
		logger.Log.Warn("Audit event channel buffer is full, dropping audit log", zap.Int64("transfer_id", transfer.ID))
	}
}

// Start starts the background processing loop. It blocks until the context is cancelled or the channel is closed.
func (w *auditWorker) Start(ctx context.Context) {
	logger.Log.Info("Background Audit Worker started successfully")
	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("Audit Worker shutting down due to context cancellation...")
			w.flush()
			return
		case transfer, ok := <-w.eventChan:
			if !ok {
				logger.Log.Info("Audit Worker event channel closed, shutting down...")
				return
			}
			w.process(transfer)
		}
	}
}

// Close closes the event channel to signal shutdown.
func (w *auditWorker) Close() {
	close(w.eventChan)
}

func (w *auditWorker) process(transfer *domain.Transfer) {
	// Simulate slow external audit, webhook delivery, or email notification
	time.Sleep(500 * time.Millisecond)

	logger.Log.Info("Asynchronous Audit Successful",
		zap.Int64("transfer_id", transfer.ID),
		zap.Int64("from_account", transfer.FromAccountID),
		zap.Int64("to_account", transfer.ToAccountID),
		zap.String("conversion", fmt.Sprintf("%.6f (%s -> %s)", transfer.ExchangeRate, transfer.SourceCurrency, transfer.TargetCurrency)),
		zap.Int64("debited_amount", transfer.SourceAmount),
		zap.Int64("credited_amount", transfer.TargetAmount),
	)
}

func (w *auditWorker) flush() {
	// Flush remaining items in channel during graceful shutdown
	for {
		select {
		case transfer, ok := <-w.eventChan:
			if !ok {
				return
			}
			w.process(transfer)
		default:
			return
		}
	}
}
