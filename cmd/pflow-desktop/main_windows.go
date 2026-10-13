//go:build windows

package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/pancake-lee/pflow"
	"github.com/pancake-lee/pflow/internal/desktop"
	plogger "github.com/pancake-lee/pgo/pkg/plogger"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	logDir := filepath.Join(home, ".pflow", "logs")
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		log.Fatal(err)
	}
	logConfig := zap.NewProductionConfig()
	logConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logPath := filepath.Join(logDir, "desktop.log")
	logConfig.OutputPaths = []string{logPath}
	logConfig.ErrorOutputPaths = []string{logPath}
	logger, err := logConfig.Build()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)
	plogger.SetDefaultLogger(plogger.FromZap(logger))

	assets, err := fs.Sub(pflow.WebDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}
	server, err := desktop.NewHandler(
		filepath.Join(home, ".pflow", "desktop.json"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	err = wails.Run(&options.App{
		Title:  "pflow",
		Width:  1280,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: server,
		},
		BackgroundColour: options.NewRGBA(16, 16, 20, 255),
		OnShutdown:       func(context.Context) { server.Close() },
	})
	if err != nil {
		plogger.Errorf("desktop startup failed: %v", err)
		os.Exit(1)
	}
}
