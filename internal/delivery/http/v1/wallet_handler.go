package v1

import (
	"github.com/gin-gonic/gin"
	"high-perf-wallet/internal/domain"
	"net/http"
	"strconv"
	"time"
)

type WalletHandler struct {
	transferUC      domain.TransferUsecase
	walletUC        domain.WalletUsecase
	idempotencyRepo domain.IdempotencyRepository
}

func NewWalletHandler(transferUC domain.TransferUsecase, walletUC domain.WalletUsecase, idempotencyRepo domain.IdempotencyRepository) *WalletHandler {
	return &WalletHandler{
		transferUC:      transferUC,
		walletUC:        walletUC,
		idempotencyRepo: idempotencyRepo,
	}
}

func (h *WalletHandler) Transfer(c *gin.Context) {
	var req struct {
		FromAccountID int64 `json:"from_account_id" binding:"required"`
		ToAccountID   int64 `json:"to_account_id" binding:"required"`
		Amount        int64 `json:"amount" binding:"required"`
	}

	idempotencyKey := c.GetHeader("X-Idempotency-Key")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request"})
		return
	}

	if idempotencyKey != "" {
		cached, err := h.idempotencyRepo.Get(c.Request.Context(), idempotencyKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency_check_failed"})
			return
		}
		if cached != nil {
			if cached.Status == "started" {
				c.JSON(http.StatusConflict, gin.H{"error": "request_in_progress"})
				return
			}
			c.Data(cached.ResponseCode, "application/json; charset=utf-8", []byte(cached.ResponseBody))
			return
		}

		err = h.idempotencyRepo.Set(c.Request.Context(), idempotencyKey, &domain.Idempotency{
			Status: "started",
		}, 24*time.Hour)
		if err != nil {
			if err.Error() == "key_already_exists" {
				c.JSON(http.StatusConflict, gin.H{"error": "request_in_progress"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "idempotency_lock_failed"})
			return
		}
	}

	err := h.transferUC.ExecuteTransfer(c.Request.Context(), idempotencyKey, req.FromAccountID, req.ToAccountID, req.Amount)

	var respCode int
	var respBody []byte
	if err != nil {
		respCode = http.StatusUnprocessableEntity
		respBody = []byte(`{"error":"` + err.Error() + `"}`)
	} else {
		respCode = http.StatusOK
		respBody = []byte(`{"status":"success","message":"funds_transferred_successfully"}`)
	}

	if idempotencyKey != "" {
		_ = h.idempotencyRepo.Set(c.Request.Context(), idempotencyKey, &domain.Idempotency{
			Status:       "completed",
			ResponseCode: respCode,
			ResponseBody: string(respBody),
		}, 24*time.Hour)
	}

	c.Data(respCode, "application/json; charset=utf-8", respBody)
}

func (h *WalletHandler) GetByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}

	acc, err := h.walletUC.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, acc)
}

func (h *WalletHandler) GetTransfers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_id"})
		return
	}

	transfers, err := h.walletUC.GetTransfers(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transfers)
}
