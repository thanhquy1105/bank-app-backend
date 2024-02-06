package api

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/thanhquy1105/simplebank/db/sqlc"
	"github.com/thanhquy1105/simplebank/gapi"
	"github.com/thanhquy1105/simplebank/token"
	"github.com/thanhquy1105/simplebank/util"
	"github.com/thanhquy1105/simplebank/val"
	"github.com/thanhquy1105/simplebank/worker"
)

type createUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,min=6"`
	FullName string `json:"full_name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

type userResponse struct {
	Username          string    `json:"username"`
	FullName          string    `json:"full_name"`
	Email             string    `json:"email"`
	PasswordChangedAt time.Time `json:"password_changed_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func newUserResponse(user db.User) userResponse {
	return userResponse{
		Username:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: user.PasswordChangedAt,
		CreatedAt:         user.CreatedAt,
	}
}

func (server *Server) createUser(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	arg := db.CreateUserTxParams{
		CreateUserParams: db.CreateUserParams{
			Username:       req.Username,
			HashedPassword: hashedPassword,
			FullName:       req.FullName,
			Email:          req.Email,
		},
		AfterCreate: func(user db.User) error {
			taskPayload := &worker.PayloadSendVerifyEmail{
				Username: user.Username,
			}
			opts := []asynq.Option{
				asynq.MaxRetry(10),
				asynq.ProcessIn(10 * time.Second),
				asynq.Queue(worker.QueueCritical),
			}
			return server.taskDistributor.DistributeTaskSendVerifyEmail(ctx, taskPayload, opts...)
		},
	}

	txResult, err := server.store.CreateUserTx(ctx, arg)
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusForbidden, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	res := newUserResponse(txResult.User)
	ctx.JSON(http.StatusOK, res)
}

type loginUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,min=6"`
}

type loginUserResponse struct {
	SessionID             uuid.UUID    `json:"session_id"`
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	User                  userResponse `json:"user"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
}

func (server *Server) loginUser(ctx *gin.Context) {
	var req loginUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	user, err := server.store.GetUser(ctx, req.Username)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	err = util.CheckPassword(req.Password, user.HashedPassword)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	accessToken, accessPayload, err := server.tokenMaker.CreateToken(
		user.Username, user.Role, server.config.AccessTokenDuration,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	refreshToken, refreshPayload, err := server.tokenMaker.CreateToken(
		user.Username, user.Role, server.config.RefreshTokenDuration,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	session, err := server.store.CreateSession(ctx, db.CreateSessionParams{
		ID:           refreshPayload.ID,
		Username:     user.Username,
		RefreshToken: refreshToken,
		UserAgent:    ctx.Request.UserAgent(),
		ClientIp:     ctx.ClientIP(),
		IsBlocked:    false,
		ExpiresAt:    refreshPayload.ExpiredAt,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	res := loginUserResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		User:                  newUserResponse(user),
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
	}

	ctx.JSON(http.StatusOK, res)
}

type updateUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateUserResponse struct {
	User userResponse `json:"user"`
}

func (server *Server) updateUser(ctx *gin.Context) {
	var req updateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Password != "" {
		if err := val.ValidatePassword(req.Password); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("password must contain from 6-100 characters")))
			return
		}
	}
	if req.Email != "" {
		if err := val.ValidateEmail(req.Email); err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if !gapi.HasPermission(authPayload.Role, []string{util.BankerRole, util.DepositorRole}) {
		ctx.JSON(http.StatusUnauthorized, errorResponse(fmt.Errorf("permission dinied")))
		return
	}
	if authPayload.Role != util.BankerRole && authPayload.Username != req.Username {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("permission dinied")))
	}

	arg := db.UpdateUserParams{
		Username: req.Username,
		FullName: pgtype.Text{
			String: req.FullName,
			Valid:  req.FullName != "",
		},
		Email: pgtype.Text{
			String: req.Email,
			Valid:  req.Email != "",
		},
		IsEmailVerified: pgtype.Bool{
			Bool:  false,
			Valid: req.Email != "",
		},
	}

	if req.Password != "" {
		hashedPassword, err := util.HashPassword(req.Password)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
		arg.HashedPassword = pgtype.Text{
			String: hashedPassword,
			Valid:  true,
		}
		arg.PasswordChangedAt = pgtype.Timestamptz{
			Time:  time.Now(),
			Valid: true,
		}
	}

	user, err := server.store.UpdateUser(ctx, arg)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return

	}
	res := newUserResponse(user)
	ctx.JSON(http.StatusOK, res)
}

func (server *Server) updateAvatar(ctx *gin.Context) {

	file, fileHeader, err := ctx.Request.FormFile("avatar")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("get form err: %s", err)))
		return
	}
	fileExt := filepath.Ext(fileHeader.Filename)
	filename := strings.Split(filepath.Base(fileHeader.Filename), ".")[0]
	now := time.Now()
	filename = strings.ReplaceAll(strings.ToLower(filename), " ", "-") + "-" + fmt.Sprintf("%v", now.Unix()) + fileExt

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	arg := db.UpdateAvatarTxParams{
		Username: authPayload.Username,
		Filename: filename,
		File:     file,
		UploadAvatar: func(file multipart.File, filename string) (string, error) {
			location, _, err := server.media.Upload(filename, file)
			if err != nil {
				return "", err
			}
			return location, nil
		},
	}

	txResult, err := server.store.UpdateAvatarTx(ctx, arg)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, txResult)

}
