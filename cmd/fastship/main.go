package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/esteban-ams/fastship/internal/config"
	"github.com/esteban-ams/fastship/internal/deploy"
	"github.com/esteban-ams/fastship/internal/webhook"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	version = "dev"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	port := flag.Int("port", 0, "Port to listen on (overrides config file)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("FastShip %s\n", version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override port if specified via flag
	if *port != 0 {
		cfg.Server.Port = *port
	}

	log.Printf("FastShip %s starting...", version)
	log.Printf("Configuration loaded from: %s", *configPath)
	log.Printf("Configured services: %d", len(cfg.Services))
	for name := range cfg.Services {
		log.Printf("  - %s", name)
	}

	// Initialize deployment engine
	engine := deploy.NewEngine(cfg)

	// Initialize webhook handler
	handler := webhook.NewHandler(cfg, engine, version)

	// Setup Echo server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// API routes
	api := e.Group("/api")
	api.POST("/deploy/:service", handler.HandleDeploy)
	api.POST("/rollback/:service", handler.HandleRollback)
	api.GET("/deployments", handler.HandleListDeployments)
	api.GET("/health", handler.HandleHealth)

	// Start server in goroutine
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server listening on %s", addr)
	log.Printf("Webhook endpoint: http://%s/api/deploy/:service", addr)

	go func() {
		var startErr error
		if cfg.Server.TLS.Enabled {
			startErr = e.StartTLS(addr, cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
		} else {
			startErr = e.Start(addr)
		}
		if startErr != nil && startErr != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", startErr)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
