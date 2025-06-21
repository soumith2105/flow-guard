package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"flowguard/internal/config"
	"flowguard/internal/limiter"
	"flowguard/internal/metrics"
	"flowguard/internal/proxy"
	"flowguard/internal/types"
)

type Config struct {
	UpstreamURL     string
	ProxyPort       string
	MetricsPort     string
	ConfigPort      string
	GRPCPort        string
}

func main() {
	// Parse command line flags and environment variables
	cfg := &Config{
		UpstreamURL: getEnvOrDefault("UPSTREAM_URL", "https://api.openai.com"),
		ProxyPort:   getEnvOrDefault("PROXY_PORT", "8080"),
		MetricsPort: getEnvOrDefault("METRICS_PORT", "9090"),
		ConfigPort:  getEnvOrDefault("CONFIG_PORT", "9091"),
		GRPCPort:    getEnvOrDefault("GRPC_PORT", "9092"),
	}

	flag.StringVar(&cfg.UpstreamURL, "upstream", cfg.UpstreamURL, "Upstream API URL")
	flag.StringVar(&cfg.ProxyPort, "proxy-port", cfg.ProxyPort, "Proxy server port")
	flag.StringVar(&cfg.MetricsPort, "metrics-port", cfg.MetricsPort, "Metrics server port")
	flag.StringVar(&cfg.ConfigPort, "config-port", cfg.ConfigPort, "REST config API port")
	flag.StringVar(&cfg.GRPCPort, "grpc-port", cfg.GRPCPort, "gRPC server port")
	flag.Parse()

	log.Printf("Starting FlowGuard with config: %+v", cfg)

	// Initialize components
	rateLimiter := limiter.NewManager()

	// Create proxy handler
	proxyHandler, err := proxy.NewHandler(cfg.UpstreamURL, rateLimiter)
	if err != nil {
		log.Fatalf("Failed to create proxy handler: %v", err)
	}

	// Create metrics collector
	metricsCollector := metrics.NewMetrics(rateLimiter)
	metricsCollector.StartMetricsUpdater(5 * time.Second)

	// Create REST API server
	restServer := config.NewRESTServer(rateLimiter)

	// Create gRPC server
	grpcServer := config.NewGRPCServer(rateLimiter)

	// Setup HTTP servers
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	// Start proxy server
	wg.Add(1)
	go func() {
		defer wg.Done()
		server := &http.Server{
			Addr:    ":" + cfg.ProxyPort,
			Handler: proxyHandler,
		}
		log.Printf("Starting proxy server on port %s", cfg.ProxyPort)
		
		go func() {
			<-ctx.Done()
			server.Shutdown(context.Background())
		}()
		
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Proxy server error: %v", err)
		}
	}()

	// Start metrics server
	wg.Add(1)
	go func() {
		defer wg.Done()
		mux := http.NewServeMux()
		mux.Handle("/metrics", metricsCollector.Handler())
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		server := &http.Server{
			Addr:    ":" + cfg.MetricsPort,
			Handler: mux,
		}
		log.Printf("Starting metrics server on port %s", cfg.MetricsPort)
		
		go func() {
			<-ctx.Done()
			server.Shutdown(context.Background())
		}()
		
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	// Start REST config server
	wg.Add(1)
	go func() {
		defer wg.Done()
		server := &http.Server{
			Addr:    ":" + cfg.ConfigPort,
			Handler: restServer,
		}
		log.Printf("Starting REST config server on port %s", cfg.ConfigPort)
		
		go func() {
			<-ctx.Done()
			server.Shutdown(context.Background())
		}()
		
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("REST config server error: %v", err)
		}
	}()

	// Start gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Starting gRPC server on port %s", cfg.GRPCPort)
		
		go func() {
			<-ctx.Done()
			grpcServer.Stop()
		}()
		
		if err := grpcServer.Start(":" + cfg.GRPCPort); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Add some default client configurations for testing
	setupDefaultClients(rateLimiter)

	log.Println("FlowGuard is running!")
	log.Printf("Proxy endpoint: http://localhost:%s", cfg.ProxyPort)
	log.Printf("Metrics endpoint: http://localhost:%s/metrics", cfg.MetricsPort)
	log.Printf("REST API endpoint: http://localhost:%s/api/v1", cfg.ConfigPort)
	log.Printf("gRPC endpoint: localhost:%s", cfg.GRPCPort)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down FlowGuard...")
	cancel()
	wg.Wait()
	log.Println("FlowGuard stopped")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func setupDefaultClients(rateLimiter *limiter.Manager) {
	// Add some example client configurations
	clients := []struct {
		clientID string
		rpm      *int64
		tpm      *int64
	}{
		{"demo-client", int64Ptr(60), int64Ptr(1000)},
		{"test-client", int64Ptr(30), int64Ptr(500)},
		{"premium-client", int64Ptr(120), int64Ptr(2000)},
	}

	for _, client := range clients {
		rateLimiter.SetClientConfig(&types.ClientConfig{
			ClientID: client.clientID,
			RPM:      client.rpm,
			TPM:      client.tpm,
			Enabled:  true,
		})
		log.Printf("Added default client: %s (RPM: %v, TPM: %v)", 
			client.clientID, *client.rpm, *client.tpm)
	}
}

func int64Ptr(i int64) *int64 {
	return &i
} 