package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/thanhquy1105/simplebank/db/sqlc"
)

type verifyEmailRequest struct {
	EmailId    int64  `form:"email_id" binding:"required,min=1"`
	SecretCode string `form:"secret_code" binding:"required,min=32,max=128"`
}

type verifyEmailResponse struct {
	IsVerified bool `json:"is_verified"`
}

func (server *Server) verifyEmail(ctx *gin.Context) {
	var req verifyEmailRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	txResult, err := server.store.VerifyEmailTx(ctx, db.VerifyEmailTxParams{
		EmailId:    req.EmailId,
		SecretCode: req.SecretCode,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	rsp := &verifyEmailResponse{
		IsVerified: txResult.User.IsEmailVerified,
	}

	ctx.JSON(http.StatusOK, rsp)
}
