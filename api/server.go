package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	db "github.com/thanhquy1105/simplebank/db/sqlc"
	"github.com/thanhquy1105/simplebank/media"
	"github.com/thanhquy1105/simplebank/token"
	"github.com/thanhquy1105/simplebank/util"
	"github.com/thanhquy1105/simplebank/worker"
	"golang.org/x/sync/errgroup"
)

// Server serves HTTP requests for our banking service.
type Server struct {
	config          util.Config
	store           db.Store
	tokenMaker      token.Maker
	taskDistributor worker.TaskDistributor
	router          *gin.Engine
	media           media.Handler
}

// NewServer creates a new HTTP server and setup routing.
func NewServer(config util.Config, store db.Store, media media.Handler, taskDistributor worker.TaskDistributor) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		taskDistributor: taskDistributor,
		media:           media,
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("currency", validCurrency)
	}

	server.setupRouter()
	return server, nil
}

func (server *Server) setupRouter() {
	router := gin.Default()

	router.POST("/users", server.createUser)
	router.POST("/users/login", server.loginUser)
	router.POST("/tokens/renew_access", server.renewAccessToken)
	router.GET("/verify_email", server.verifyEmail)

	authRoutes := router.Group("/").Use(authMiddleware(server.tokenMaker))
	authRoutes.POST("/users/update", server.updateUser)
	authRoutes.POST("/users/avatar", server.updateAvatar)

	authRoutes.POST("/accounts", server.createAccount)
	authRoutes.GET("/accounts/:id", server.getAccount)
	authRoutes.GET("/accounts", server.listAccount)

	authRoutes.POST("/transfers", server.createTransfer)

	server.router = router
}

// Start runs the HTTP server on a specific address
func (server *Server) Start(ctx context.Context, waitGroup *errgroup.Group, address string) error {
	srv := &http.Server{
		Addr:    address,
		Handler: server.router,
	}
	var err error
	waitGroup.Go(func() error {
		err = srv.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Fatal().Msg("RESTFUL API server failed to serve")
			return err
		}
		log.Info().Msgf("RESTFUL API server server at %s", address)
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown RESTFUL API server")

		err = srv.Shutdown(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("failed to shutdown RESTFUL API server")
			return err
		}
		log.Info().Msg("RESTFUL API server is stopped")
		return nil
	})

	return err
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
