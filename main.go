package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/thanhquy1105/simplebank/api"
	db "github.com/thanhquy1105/simplebank/db/sqlc"
	"github.com/thanhquy1105/simplebank/gapi"
	"github.com/thanhquy1105/simplebank/mail"
	"github.com/thanhquy1105/simplebank/media"
	_ "github.com/thanhquy1105/simplebank/media/fs"
	_ "github.com/thanhquy1105/simplebank/media/s3"
	"github.com/thanhquy1105/simplebank/pb"
	"github.com/thanhquy1105/simplebank/util"
	"github.com/thanhquy1105/simplebank/worker"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:embed doc/swagger/*
var staticAssets embed.FS

var interruptSignals = []os.Signal{os.Interrupt, syscall.SIGINT, syscall.SIGTERM}

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Msg("cannot load config")
	}

	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stop()

	connPool, err := pgxpool.New(ctx, config.DBSource)
	if err != nil {
		log.Fatal().Msg("cannot connect to db")
	}

	store := db.NewStore(connPool)

	redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}
	taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

	mediaConfig, err := util.LoadMediaConfig(".")
	if err != nil {
		log.Fatal().Msg(fmt.Sprintln("cannot load media config", err))
	}

	media, err := media.UseMediaHandler(mediaConfig)
	if err != nil {
		log.Fatal().Msg(fmt.Sprintln("Failed to init media handler: ", err))
	}

	waitGroup, ctx := errgroup.WithContext(ctx)

	runTaskProcessor(ctx, waitGroup, config, redisOpt, store, media)
	//runGinServer(ctx, waitGroup, config, store, media, taskDistributor)
	runGatewayServer(ctx, waitGroup, config, store, taskDistributor)
	runGrpcServer(ctx, waitGroup, config, store, taskDistributor)

	err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("error from wait group")
	}
}

func runTaskProcessor(ctx context.Context, waitGroup *errgroup.Group, config util.Config, redisOpt asynq.RedisClientOpt, store db.Store, media media.Handler) {
	mailer := mail.NewGmailSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, mailer, media)

	log.Info().Msg("start task processor")
	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown ")

		taskProcessor.Shutdown()
		log.Info().Msg("task processor is stopped")
		return nil
	})
}

func runGrpcServer(ctx context.Context, waitGroup *errgroup.Group, config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Msg("cannot create server")
	}

	grpcLogger := grpc.UnaryInterceptor(gapi.GrpcLogger)

	grpcServer := grpc.NewServer(grpcLogger)
	pb.RegisterSimpleBankServer(grpcServer, server)

	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", config.GRPCServerAddress)
	if err != nil {
		log.Fatal().Msg("cannot create listener")
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
		err = grpcServer.Serve(listener)
		if err != nil {
			if errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
			log.Error().Err(err).Msg("gRPC server failed to serve")
			return err
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown gRPC server")

		grpcServer.GracefulStop()
		log.Info().Msg("gRPC server is stopped")
		return nil
	})
}

func runGatewayServer(ctx context.Context, waitGroup *errgroup.Group, config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := gapi.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Msg("cannot create server")
	}

	jsonOption := runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
		MarshalOptions: protojson.MarshalOptions{
			UseProtoNames:   true,
			EmitUnpopulated: true,
		},
		UnmarshalOptions: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	})

	grpcMux := runtime.NewServeMux(jsonOption)

	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
	if err != nil {
		log.Fatal().Msg("cannot register handler server")
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	assets, _ := fs.Sub(staticAssets, "doc")
	mux.Handle("/swagger/", http.FileServer(http.FS(assets)))

	httpServer := &http.Server{
		Handler: gapi.HttpLogger(mux),
		Addr:    config.HTTPServerAddress,
	}

	waitGroup.Go(func() error {
		log.Info().Msgf("start HTTP gateway server at %s", httpServer.Addr)
		err = httpServer.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			log.Error().Err(err).Msg("HTTP gateway server failed to serve")
			return err
		}
		return nil
	})

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown HTTP gateway server")

		err := httpServer.Shutdown(context.Background())
		if err != nil {
			log.Error().Err(err).Msg("failed to shutdown HTTP gateway server")
			return err
		}

		log.Info().Msg("HTTP gateway server is stopped")
		return nil
	})
}

func runGinServer(ctx context.Context, waitGroup *errgroup.Group, config util.Config, store db.Store, media media.Handler, taskDistributor worker.TaskDistributor) {
	ginServer, err := api.NewServer(config, store, media, taskDistributor)
	if err != nil {
		log.Fatal().Msg("cannot create server")
	}

	err = ginServer.Start(ctx, waitGroup, config.HTTPServerAddress)
	if err != nil {
		log.Fatal().Msg("cannot start server")
	}

}
