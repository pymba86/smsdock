package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"smsdock/packages/api/internal/httpapi"
	"smsdock/packages/api/internal/modem"
	"smsdock/packages/api/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	config := loadConfig()
	if err := os.MkdirAll(filepath.Dir(config.dbPath), 0o755); err != nil {
		log.Fatalf("create db dir: %v", err)
	}

	store, err := storage.Open(ctx, config.dbPath)
	if err != nil {
		log.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	discovery := modem.NewSerialDiscovery(config.deviceGlobs)
	manager := modem.NewManager(store, discovery)
	if err := manager.Load(ctx); err != nil {
		log.Fatalf("load manager: %v", err)
	}
	defer manager.StopAll()

	server := &http.Server{
		Addr:              config.httpAddr,
		Handler:           httpapi.New(store, manager).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go cleanupLoop(ctx, manager)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("smsdock api listening on %s", config.httpAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen and serve: %v", err)
	}
}

type appConfig struct {
	httpAddr    string
	dbPath      string
	deviceGlobs []string
}

func loadConfig() appConfig {
	return appConfig{
		httpAddr: envOrDefault("SMSDOCK_HTTP_ADDR", ":8080"),
		dbPath:   envOrDefault("SMSDOCK_DB_PATH", "./data/smsdock.db"),
		deviceGlobs: splitCSV(
			envOrDefault("SMSDOCK_DEVICE_GLOBS", "/dev/serial/by-id/*,/dev/ttyUSB*"),
		),
	}
}

func cleanupLoop(ctx context.Context, manager *modem.Manager) {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := manager.CleanupEvents(cleanupCtx); err != nil {
				log.Printf("cleanup events: %v", err)
			}
			cancel()
		}
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		panic(fmt.Sprintf("empty csv config: %s", value))
	}
	return result
}
