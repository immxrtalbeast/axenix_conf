package main

import (
	"log/slog"
	"os"

	httpapi "github.com/immxrtalbeast/axenix_conf/internal/api/http"
	"github.com/immxrtalbeast/axenix_conf/internal/config"
	"github.com/immxrtalbeast/axenix_conf/internal/repository"
	"github.com/immxrtalbeast/axenix_conf/internal/service"
	"github.com/immxrtalbeast/axenix_conf/lib/logger/slogpretty"
	"github.com/joho/godotenv"
	"github.com/pion/webrtc/v3"
)

func main() {
	_ = godotenv.Load(".env")

	cfg := config.MustLoad()
	log := setupLogger(cfg.Env)

	rtcConfig := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: cfg.WebRTC.STUNServers},
		},
	}

	roomRepo := repository.NewInMemoryRoomRepository()
	userRepo := repository.NewInMemoryUserRepository()

	roomService := service.NewRoomService(roomRepo, userRepo, rtcConfig, log)
	userService := service.NewUserService(userRepo, log)

	roomController := httpapi.NewRoomController(roomService, userService)
	userController := httpapi.NewUserController(userService)

	router := httpapi.SetupRouter(roomController, userController)

	log.Info("starting application", slog.String("addr", cfg.HTTP.Address))
	if err := router.Run(cfg.HTTP.Address); err != nil {
		log.Error("http server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

const (
	envLocal = "local"
	envDev   = "dev"
	envProd  = "prod"
)

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = setupPrettySlog()
	case envDev:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		)
	default:
		log = setupPrettySlog()
	}

	return log
}

func setupPrettySlog() *slog.Logger {
	opts := slogpretty.PrettyHandlerOptions{
		SlogOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}

	handler := opts.NewPrettyHandler(os.Stdout)

	return slog.New(handler)
}
