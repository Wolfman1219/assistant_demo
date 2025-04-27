package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// AppConfig holds the application configuration
type AppConfig struct {
	VadServiceAddr     string
	TriggerServiceAddr string
	SttServiceAddr     string
	TtsServiceAddr     string
	LlmServiceAddr     string
}

// App represents the main application
type App struct {
	config        AppConfig
	vadClient     VadClient
	triggerClient TriggerClient
	sttClient     SttClient
	llmClient     LlmClient
	ttsClient     TtsClient
	upgrader      websocket.Upgrader
	clients       map[*websocket.Conn]*ClientState
	clientsMutex  sync.Mutex
}

// NewApp creates a new application instance
func NewApp(config AppConfig) *App {
	// Create the upgrader with CheckOrigin disabled for development
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all connections in development
		},
	}

	// Initialize the app
	app := &App{
		config:       config,
		upgrader:     upgrader,
		clients:      make(map[*websocket.Conn]*ClientState),
		clientsMutex: sync.Mutex{},
	}

	// Initialize clients for the AI services
	var err error

	// Initialize VAD client
	app.vadClient, err = NewVadClient(config.VadServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to VAD service: %v\n", err)
	}

	// Initialize Trigger client
	app.triggerClient, err = NewTriggerClient(config.TriggerServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to Trigger service: %v\n", err)
	}

	// Initialize STT client
	app.sttClient, err = NewSttClient(config.SttServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to STT service: %v\n", err)
	}

	// Initialize LLM client
	app.llmClient = NewLlmClient(config.LlmServiceAddr)

	// Initialize TTS client
	app.ttsClient, err = NewTtsClient(config.TtsServiceAddr)
	if err != nil {
		log.Printf("Warning: Failed to connect to TTS service: %v\n", err)
	}

	return app
}

// Routes returns the router for the application
func (app *App) Routes() http.Handler {
	r := mux.NewRouter()

	// Static files route
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// WebSocket route
	r.HandleFunc("/ws", app.handleWebSocket)

	// Home route serves the index.html
	r.HandleFunc("/", app.handleHome)

	return r
}

// handleHome serves the index.html file
func (app *App) handleHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/index.html")
}

// handleWebSocket handles WebSocket connections
func (app *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := app.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}

	// Create a new client state
	clientState := NewClientState(conn, app)

	// Add the client to the map
	app.clientsMutex.Lock()
	app.clients[conn] = clientState
	app.clientsMutex.Unlock()

	// Start handling the client
	go clientState.handleClient()
}

// Close closes all connections and resources
func (app *App) Close() error {
	// Close all client connections
	app.clientsMutex.Lock()
	for conn, client := range app.clients {
		client.close()
		delete(app.clients, conn)
	}
	app.clientsMutex.Unlock()

	// Close service clients
	if app.vadClient != nil {
		app.vadClient.Close()
	}
	if app.triggerClient != nil {
		app.triggerClient.Close()
	}
	if app.sttClient != nil {
		app.sttClient.Close()
	}
	if app.ttsClient != nil {
		app.ttsClient.Close()
	}

	return nil
}

// removeClient removes a client from the clients map
func (app *App) removeClient(conn *websocket.Conn) {
	app.clientsMutex.Lock()
	defer app.clientsMutex.Unlock()

	if client, ok := app.clients[conn]; ok {
		client.close()
		delete(app.clients, conn)
	}
}
