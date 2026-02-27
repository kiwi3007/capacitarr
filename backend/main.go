package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/jobs"
	"capacitarr/internal/logger"
	"capacitarr/internal/poller"
	"capacitarr/routes"
)

//go:embed frontend/dist/*
var embeddedFiles embed.FS

// getFileSystem strips the "frontend/dist" prefix from the embedded filesystem
func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(embeddedFiles, "frontend/dist")
	if err != nil {
		panic(fmt.Errorf("error stripping prefix from embedded fs: %v", err))
	}
	return http.FS(fsys)
}

func main() {
	cfg := config.Load()
	logger.Init(cfg.Debug)

	slog.Info("Starting Capacitarr backend", "port", cfg.Port, "base_url", cfg.BaseURL)

	if err := db.Init(cfg); err != nil {
		slog.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Initialize background jobs
	poller.Start(15 * time.Second) // Poll frequently to simulate active capacity ingestion
	jobs.Start()

	// Initialize Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Add CORS middleware
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// API Routing group mapping to /api/v1
	// Respect configuration's BaseURL for any proxy magic
	prefix := cfg.BaseURL
	if prefix == "/" {
		prefix = "" // allow mapping directly to routes without double slashing
	}
	apiGroup := e.Group(prefix + "/api/v1")
	routes.RegisterAPIRoutes(apiGroup, db.DB, cfg)

	// Serve the embedded Nuxt static frontend
	// This uses echo.WrapHandler to serve the filesystem directly
	frontendHandler := http.FileServer(getFileSystem())

	if cfg.BaseURL != "" && cfg.BaseURL != "/" {
		// Serve UI mapped onto BaseURL
		uiGroup := e.Group(cfg.BaseURL)

		// Map /* onto the filesystem
		uiGroup.GET("/*", echo.WrapHandler(http.StripPrefix(cfg.BaseURL, frontendHandler)))
	} else {
		// Serve UI mapped onto root
		e.GET("/*", echo.WrapHandler(frontendHandler))
	}

	// Start Server
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}

