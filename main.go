package main

import (
	"context"
	"embed"
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
	_ "github.com/thanhquy1105/simplebank/media/s3"
	"github.com/thanhquy1105/simplebank/pb"
	"github.com/thanhquy1105/simplebank/util"
	"github.com/thanhquy1105/simplebank/worker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:embed doc/swagger/*
var staticAssets embed.FS
var (
	ginServer       *api.Server
	gServer         net.Listener
	gGateway        net.Listener
	taskDistributor worker.TaskDistributor
	taskProcessor   worker.TaskProcessor
)

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Msg("cannot load config")
	}

	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	connPool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal().Msg("cannot connect to db")
	}

	store := db.NewStore(connPool)

	redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}
	taskDistributor = worker.NewRedisTaskDistributor(redisOpt)

	mediaConfig, err := util.LoadMediaConfig(".")
	if err != nil {
		log.Fatal().Msg(fmt.Sprintln("cannot load media config", err))
	}

	media, err := media.UseMediaHandler(mediaConfig)
	if err != nil {
		log.Fatal().Msg(fmt.Sprintln("Failed to init media handler: ", err))
	}

	// listen for signals
	// go listenForShutdown()

	go runTaskProcessor(config, redisOpt, store, media)
	runGinServer(config, store, media, taskDistributor)
	// go runGatewayServer(config, store, taskDistributor)
	// runGrpcServer(config, store, taskDistributor)
}

func runTaskProcessor(config util.Config, redisOpt asynq.RedisClientOpt, store db.Store, media media.Handler) {
	mailer := mail.NewGmailSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor = worker.NewRedisTaskProcessor(redisOpt, store, mailer, media)
	log.Info().Msg("start task processor")
	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}
}

func runGrpcServer(config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
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
	gServer = listener

	log.Info().Msgf("start gRPC server at %s", listener.Addr().String())
	err = grpcServer.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal().Msg("cannot start gRPC server")
	}
}

func runGatewayServer(config util.Config, store db.Store, taskDistributor worker.TaskDistributor) {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = pb.RegisterSimpleBankHandlerServer(ctx, grpcMux, server)
	if err != nil {
		log.Fatal().Msg("cannot register handler server")
	}

	mux := http.NewServeMux()
	mux.Handle("/", grpcMux)

	assets, _ := fs.Sub(staticAssets, "doc")
	mux.Handle("/swagger/", http.FileServer(http.FS(assets)))

	listener, err := net.Listen("tcp", config.HTTPServerAddress)
	if err != nil {
		log.Fatal().Msg("cannot create listener")
	}
	gGateway = listener

	log.Info().Msgf("start HTTP gateway server at %s", listener.Addr().String())
	handler := gapi.HttpLogger(mux)
	err = http.Serve(listener, handler)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal().Msg("cannot start HTTP gateway server")
	}
}

func listenForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("closing channels and shutting down application...")

	log.Info().Msg("Close another connection")
	taskDistributor.Close()
	taskProcessor.Shutdown()

	log.Info().Msg("Stop grpc server and http gateway server")

	//err := gServer.Close()
	//err1 := gGateway.Close()
	// if err != nil || err1 != nil {
	// 	log.Error().Msg("Stop grpc server and http gateway server")
	// }

	close(quit)
	os.Exit(0)
}

func runGinServer(config util.Config, store db.Store, media media.Handler, taskDistributor worker.TaskDistributor) {
	ginServer, err := api.NewServer(config, store, media, taskDistributor)
	if err != nil {
		log.Fatal().Msg("cannot create server")
	}

	err = ginServer.Start(config.HTTPServerAddress)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal().Msg("cannot start server")
	}
}
