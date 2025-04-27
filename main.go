package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	// Load configuration from .env file if present
	_ = godotenv.Load()

	// Command line flags
	port := flag.String("port", getEnv("PORT", "8080"), "HTTP server port")
	vadService := flag.String("vad", getEnv("VAD_SERVICE", "localhost:50051"), "VAD gRPC service address")
	triggerService := flag.String("trigger", getEnv("TRIGGER_SERVICE", "localhost:50052"), "Trigger detection gRPC service address")
	sttService := flag.String("stt", getEnv("STT_SERVICE", "localhost:50053"), "STT gRPC service address")
	ttsService := flag.String("tts", getEnv("TTS_SERVICE", "localhost:50054"), "TTS gRPC service address")
	llmService := flag.String("llm", getEnv("LLM_SERVICE", "http://localhost:8000"), "LLM HTTP service address")

	flag.Parse()

	// Initialize the application
	app := NewApp(AppConfig{
		VadServiceAddr:     *vadService,
		TriggerServiceAddr: *triggerService,
		SttServiceAddr:     *sttService,
		TtsServiceAddr:     *ttsService,
		LlmServiceAddr:     *llmService,
	})

	// Create an HTTP server
	server := &http.Server{
		Addr:    ":" + *port,
		Handler: app.Routes(),
	}

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on port %s\n", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("Shutting down server...")

	// Create a deadline to wait for current operations to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Gracefully shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v\n", err)
	}

	if err := app.Close(); err != nil {
		log.Fatalf("Error closing application: %v\n", err)
	}

	log.Println("Server exited properly")
}

// Helper function to get environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
