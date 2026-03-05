package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/esteban-ams/deploydeck/internal/config"
	"github.com/esteban-ams/deploydeck/internal/deploy"
	"github.com/esteban-ams/deploydeck/internal/ratelimit"
	"github.com/esteban-ams/deploydeck/internal/webhook"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/cobra"
)

var (
	version    = "dev"
	configPath string
	port       int
)

var rootCmd = &cobra.Command{
	Use:   "deploydeck",
	Short: "Your container deployment command center",
	Long:  "DeployDeck listens for webhooks from CI/CD pipelines and orchestrates Docker Compose deployments with health checking and rollback support.",
	RunE:  runServer,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "Path to configuration file")
	rootCmd.Flags().IntVar(&port, "port", 0, "Port to listen on (overrides config file)")
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if port != 0 {
		cfg.Server.Port = port
	}

	log.Printf("DeployDeck %s starting...", version)
	log.Printf("Configuration loaded from: %s", configPath)
	log.Printf("Configured services: %d", len(cfg.Services))
	for name := range cfg.Services {
		log.Printf("  - %s", name)
	}

	engine := deploy.NewEngine(cfg)
	handler := webhook.NewHandler(cfg, engine, version)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	api := e.Group("/api")
	api.GET("/deployments", handler.HandleListDeployments)
	api.GET("/health", handler.HandleHealth)

	// Rate limiting applies only to the mutating webhook endpoints.
	webhookGroup := api.Group("")
	if cfg.RateLimit.Enabled {
		log.Printf("Rate limiting enabled: %d requests/min per IP (burst: %d)",
			cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.BurstSize)
		rl := ratelimit.NewLimiter(cfg.RateLimit.RequestsPerMinute, cfg.RateLimit.BurstSize)
		webhookGroup.Use(rl.Middleware())
	}
	webhookGroup.POST("/deploy/:service", handler.HandleDeploy)
	webhookGroup.POST("/rollback/:service", handler.HandleRollback)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	log.Println("Server stopped gracefully")
	return nil
}
