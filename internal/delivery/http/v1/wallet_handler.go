package v1

import (
	"errors"
	"github.com/gin-gonic/gin"
	"high-perf-wallet/internal/domain"
	"high-perf-wallet/pkg/apperror"
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

// handleError maps an AppError to correct HTTP status codes and formatted responses
func handleError(c *gin.Context, err error) {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		var status int
		switch appErr.Type {
		case apperror.TypeValidation:
			status = http.StatusBadRequest
		case apperror.TypeNotFound:
			status = http.StatusNotFound
		case apperror.TypeConflict:
			status = http.StatusConflict
		default:
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	// Fallback for non-AppError
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    "INTERNAL_SERVER_ERROR",
			"message": "An unexpected error occurred",
		},
	})
}

func (h *WalletHandler) Transfer(c *gin.Context) {
	var req struct {
		FromAccountID int64 `json:"from_account_id" binding:"required"`
		ToAccountID   int64 `json:"to_account_id" binding:"required"`
		Amount        int64 `json:"amount" binding:"required"`
	}

	idempotencyKey := c.GetHeader("X-Idempotency-Key")

	if err := c.ShouldBindJSON(&req); err != nil {
		handleError(c, apperror.NewValidationError("BAD_REQUEST", "Invalid request body or missing fields"))
		return
	}

	if idempotencyKey != "" {
		cached, err := h.idempotencyRepo.Get(c.Request.Context(), idempotencyKey)
		if err != nil {
			handleError(c, apperror.NewInternalError("IDEMPOTENCY_ERROR", "Failed to check idempotency status", err))
			return
		}
		if cached != nil {
			if cached.Status == "started" {
				handleError(c, apperror.NewConflictError("REQUEST_IN_PROGRESS", "A transaction with this idempotency key is already in progress"))
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
				handleError(c, apperror.NewConflictError("REQUEST_IN_PROGRESS", "A transaction with this idempotency key is already in progress"))
				return
			}
			handleError(c, apperror.NewInternalError("IDEMPOTENCY_ERROR", "Failed to establish transaction lock", err))
			return
		}
	}

	err := h.transferUC.ExecuteTransfer(c.Request.Context(), idempotencyKey, req.FromAccountID, req.ToAccountID, req.Amount)

	var respCode int
	var respBody []byte
	if err != nil {
		var appErr *apperror.AppError
		if errors.As(err, &appErr) {
			switch appErr.Type {
			case apperror.TypeValidation:
				respCode = http.StatusBadRequest
			case apperror.TypeNotFound:
				respCode = http.StatusNotFound
			case apperror.TypeConflict:
				respCode = http.StatusConflict
			default:
				respCode = http.StatusInternalServerError
			}
			respBody = []byte(`{"error":{"code":"` + appErr.Code + `","message":"` + appErr.Message + `"}}`)
		} else {
			respCode = http.StatusInternalServerError
			respBody = []byte(`{"error":{"code":"INTERNAL_SERVER_ERROR","message":"An unexpected error occurred"}}`)
		}
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
		handleError(c, apperror.NewValidationError("INVALID_ID", "The account ID must be a valid number"))
		return
	}

	acc, err := h.walletUC.GetByID(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, acc)
}

func (h *WalletHandler) GetTransfers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		handleError(c, apperror.NewValidationError("INVALID_ID", "The account ID must be a valid number"))
		return
	}

	// Set default values for pagination
	limit := 10
	offset := 0

	// Parse limit query parameter
	if limitStr := c.Query("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		} else if err != nil {
			handleError(c, apperror.NewValidationError("INVALID_LIMIT", "The limit parameter must be a positive number"))
			return
		}
	}
	// Clamp limit to a maximum of 100 to protect server resources
	if limit > 100 {
		limit = 100
	}

	// Parse offset query parameter
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if val, err := strconv.Atoi(offsetStr); err == nil && val >= 0 {
			offset = val
		} else if err != nil {
			handleError(c, apperror.NewValidationError("INVALID_OFFSET", "The offset parameter must be a non-negative number"))
			return
		}
	}

	paginatedTransfers, err := h.walletUC.GetTransfers(c.Request.Context(), id, limit, offset)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, paginatedTransfers)
}
