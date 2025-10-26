package main

import (
	"errors"
	"log/slog"
	"os"
	"time"

	httpapi "github.com/immxrtalbeast/axenix_conf/internal/api/http"
	"github.com/immxrtalbeast/axenix_conf/internal/config"
	"github.com/immxrtalbeast/axenix_conf/internal/repository"
	"github.com/immxrtalbeast/axenix_conf/internal/repository/model"
	"github.com/immxrtalbeast/axenix_conf/internal/service"
	"github.com/immxrtalbeast/axenix_conf/lib/logger/slogpretty"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load(".env")

	cfg := config.MustLoad()
	log := setupLogger(cfg.Env)

	db, err := connectDatabase(cfg.Database)
	if err != nil {
		log.Error("failed to connect database", slog.Any("error", err))
		os.Exit(1)
	}

	roomRepo := repository.NewPostgresRoomRepository(db)
	userRepo := repository.NewPostgresUserRepository(db)

	roomService := service.NewRoomService(roomRepo, userRepo, log)
	userService := service.NewUserService(userRepo, log)

	roomController := httpapi.NewRoomController(roomService, userService, log)
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

func connectDatabase(cfg config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, errors.New("database dsn is empty")
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&model.Room{}, &model.Peer{}, &model.User{})

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}
