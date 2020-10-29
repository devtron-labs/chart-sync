package main

import (
	"github.com/devtron-labs/chart-sync/pkg"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
)

type App struct {
	Logger      *zap.SugaredLogger
	db          *pg.DB
	syncService pkg.SyncService
}

func NewApp(Logger *zap.SugaredLogger,
	db *pg.DB,
	syncService pkg.SyncService) *App {
	return &App{
		Logger:      Logger,
		db:          db,
		syncService: syncService,
	}
}

func (app *App) Start() {
	_, err := app.syncService.Sync()
	if err != nil {
		app.Logger.Errorw("err", "err", err)
	}
}
