package main

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ClientState represents the state of a client connection
type ClientState struct {
	conn             *websocket.Conn
	app              *App
	state            State
	stateMutex       sync.Mutex
	cancelFuncs      map[string]context.CancelFunc
	cancelMutex      sync.Mutex
	transcript       string
	vadActive        bool
	triggered        bool
	audioBuffer      [][]byte
	audioBufferMutex sync.Mutex
	closed           bool
	closeMutex       sync.Mutex
}

// State represents the possible states of the client
type State string

const (
	StateIdle         State = "IDLE"
	StateDetecting    State = "LISTENING"
	StateTriggered    State = "TRIGGERED"
	StateProcessing   State = "PROCESSING"
	StateSpeaking     State = "SPEAKING"
	StateError        State = "ERROR"
	StateDisconnected State = "DISCONNECTED"
)

// StatusMessage represents a status update to send to the client
type StatusMessage struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// TranscriptMessage represents a transcript update to send to the client
type TranscriptMessage struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	IsFinal bool   `json:"isFinal"`
}

// ResponseMessage represents an LLM response to send to the client
type ResponseMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewClientState creates a new client state
func NewClientState(conn *websocket.Conn, app *App) *ClientState {
	return &ClientState{
		conn:        conn,
		app:         app,
		state:       StateIdle,
		cancelFuncs: make(map[string]context.CancelFunc),
		audioBuffer: make([][]byte, 0),
		closed:      false,
	}
}

// handleClient handles the WebSocket connection for a client
// Update in handleClient in client_state.go

// handleClient handles the WebSocket connection for a client
func (cs *ClientState) handleClient() {
	defer func() {
		cs.app.removeClient(cs.conn)
		log.Println("Client disconnected")
	}()

	// Send initial status
	cs.sendStatus(StateIdle, "Ready")

	// Start processing VAD events
	cs.startProcessingVadEvents()

	// Handle incoming messages
	for {
		// Read message from WebSocket
		messageType, message, err := cs.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle binary messages (audio data)
		if messageType == websocket.BinaryMessage {
			cs.handleAudioData(message)
		} else if messageType == websocket.TextMessage {
			// Handle text messages (commands)
			cs.handleTextCommand(string(message))
		}
	}
}

// handleAudioData processes incoming audio data
func (cs *ClientState) handleAudioData(audioData []byte) {
	// Make a copy of the audio data
	dataCopy := make([]byte, len(audioData))
	copy(dataCopy, audioData)

	// Store audio in buffer for STT if needed
	if cs.getState() == StateTriggered {
		cs.audioBufferMutex.Lock()
		cs.audioBuffer = append(cs.audioBuffer, dataCopy)
		cs.audioBufferMutex.Unlock()
	}

	// Always send audio to VAD
	if cs.app.vadClient != nil {
		err := cs.app.vadClient.ProcessAudio(dataCopy)
		if err != nil {
			log.Printf("Error sending audio to VAD: %v", err)
		}
	}
}

// handleTextCommand processes text commands from the client
func (cs *ClientState) handleTextCommand(command string) {
	var cmd map[string]string
	err := json.Unmarshal([]byte(command), &cmd)
	if err != nil {
		log.Printf("Invalid command format: %v", err)
		return
	}

	switch cmd["action"] {
	case "reset":
		cs.resetState()
	case "stop":
		cs.cancelAllOperations()
		cs.resetState()
	}
}

// Add to client_state.go

// startProcessingVadEvents starts processing VAD events
func (cs *ClientState) startProcessingVadEvents() {
	if cs.app.vadClient == nil {
		log.Println("VAD client is not available")
		return
	}

	// Get the event channel
	eventChan := cs.app.vadClient.GetEventChannel()

	// Start a goroutine to process VAD events
	go func() {
		for {
			select {
			case event, ok := <-eventChan:
				if !ok {
					// Channel closed
					log.Println("VAD event channel closed not ok")
					return
				}

				// Process the VAD event
				switch event.Type {
				case "start":
					cs.vadActive = true
					log.Printf("VAD event: Speech started - %s", event.Message)

					// If we're in IDLE state, check for trigger
					if cs.getState() == StateIdle {
						// This is where we would trigger wake word detection
						// For now, let's just simulate a trigger with a probability
						if cs.app.triggerClient != nil && cs.app.triggerClient.IsTriggered(nil) {
							cs.triggered = true
							cs.setState(StateTriggered)
							cs.sendStatus(StateTriggered, "Listening to you...")

							// Clear the audio buffer to start fresh
							cs.audioBufferMutex.Lock()
							cs.audioBuffer = make([][]byte, 0)
							cs.audioBufferMutex.Unlock()
						}
					}

				case "end":
					cs.vadActive = false
					log.Printf("VAD event: Speech ended - %s", event.Message)

					// If we're in TRIGGERED state, process the collected audio
					if cs.getState() == StateTriggered {
						go cs.processAudio()
					}

				case "continue":
					// Just log for debugging
					log.Printf("VAD event: Speech continuing - %s", event.Message)
				}
			}
		}
	}()
}

// startVadTriggerDetection starts the parallel VAD/Trigger detection process
func (cs *ClientState) startVadTriggerDetection() {
	ctx, cancel := context.WithCancel(context.Background())
	cs.addCancelFunc("vadTrigger", cancel)

	go func() {
		defer cs.removeCancelFunc("vadTrigger")

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Check if we have audio data to process
				cs.audioBufferMutex.Lock()
				if len(cs.audioBuffer) == 0 {
					cs.audioBufferMutex.Unlock()
					time.Sleep(10 * time.Millisecond)
					continue
				}

				// Get the oldest audio chunk
				audioChunk := cs.audioBuffer[0]
				cs.audioBuffer = cs.audioBuffer[1:]
				cs.audioBufferMutex.Unlock()

				// Process the audio chunk
				cs.processVadTrigger(audioChunk)
			}
		}
	}()
}

// processVadTrigger processes audio for VAD and trigger detection
func (cs *ClientState) processVadTrigger(audioData []byte) {
	currentState := cs.getState()

	// Skip trigger detection if we're already in a triggered state or beyond
	if currentState == StateTriggered || currentState == StateProcessing || currentState == StateSpeaking {
		return
	}

	// Check voice activity
	if cs.app.vadClient != nil {
		isActive := cs.app.vadClient.IsActive(audioData)

		// Update VAD status if changed
		if isActive != cs.vadActive {
			cs.vadActive = isActive

			if isActive {
				// Voice activity detected, now check for trigger
				if cs.app.triggerClient != nil {
					isTriggered := cs.app.triggerClient.IsTriggered(audioData)
					if isTriggered {
						// Wake word detected
						cs.triggered = true
						cs.setState(StateTriggered)
						cs.sendStatus(StateTriggered, "Listening to you...")

						// Clear the audio buffer to start fresh
						cs.audioBufferMutex.Lock()
						cs.audioBuffer = make([][]byte, 0)
						cs.audioBufferMutex.Unlock()
					}
				}
			}
		}
	}
}

// processAudio processes the collected audio with STT and LLM
func (cs *ClientState) processAudio() {
	// Change state to processing
	cs.setState(StateProcessing)
	cs.sendStatus(StateProcessing, "Processing your request...")

	// Create context for the operation
	ctx, cancel := context.WithCancel(context.Background())
	cs.addCancelFunc("processing", cancel)
	defer cs.removeCancelFunc("processing")

	// Get the audio buffer
	cs.audioBufferMutex.Lock()
	audioBuffer := cs.audioBuffer
	cs.audioBuffer = make([][]byte, 0) // Clear the buffer
	cs.audioBufferMutex.Unlock()

	// Transcribe the audio
	if cs.app.sttClient == nil {
		cs.sendStatus(StateError, "STT service unavailable")
		cs.setState(StateIdle)
		return
	}

	transcript, err := cs.app.sttClient.Transcribe(ctx, audioBuffer)
	if err != nil {
		log.Printf("STT error: %v", err)
		cs.sendStatus(StateError, "Failed to transcribe audio")
		cs.setState(StateIdle)
		return
	}

	// Send the transcript to the client
	cs.transcript = transcript
	cs.sendTranscript(transcript, true)

	// Send the transcript to the LLM service
	if cs.app.llmClient == nil {
		cs.sendStatus(StateError, "LLM service unavailable")
		cs.setState(StateIdle)
		return
	}

	responseStream, err := cs.app.llmClient.GetResponse(ctx, transcript)
	if err != nil {
		log.Printf("LLM error: %v", err)
		cs.sendStatus(StateError, "Failed to get AI response")
		cs.setState(StateIdle)
		return
	}

	// Change state to speaking
	cs.setState(StateSpeaking)
	cs.sendStatus(StateSpeaking, "Speaking...")

	// Process the streaming response
	var fullResponse string
	var currentSentence string
	for {
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-responseStream:
			if !ok {
				// End of stream, synthesize last sentence if any
				if currentSentence != "" {
					cs.synthesizeAndSend(ctx, currentSentence)
				}
				// Reset state to idle
				cs.setState(StateIdle)
				cs.sendStatus(StateIdle, "Ready")
				return
			}

			fullResponse += resp
			currentSentence += resp

			// Check if we have a complete sentence
			if strings.Contains(currentSentence, ".") || strings.Contains(currentSentence, "!") || strings.Contains(currentSentence, "?") {
				// Find the end of the sentence
				endIdx := strings.LastIndexAny(currentSentence, ".!?") + 1

				if endIdx > 0 && endIdx < len(currentSentence) {
					sentence := currentSentence[:endIdx]
					currentSentence = currentSentence[endIdx:]

					// Synthesize and send the sentence
					cs.synthesizeAndSend(ctx, sentence)
				}
			}

			// Send the incremental response to the client
			cs.sendResponse(resp)
		}
	}
}

// synthesizeAndSend synthesizes a text sentence and sends it to the client
func (cs *ClientState) synthesizeAndSend(ctx context.Context, text string) {
	if cs.app.ttsClient == nil {
		return
	}

	// Synthesize the text
	audioData, err := cs.app.ttsClient.Synthesize(ctx, text)
	if err != nil {
		log.Printf("TTS error: %v", err)
		return
	}

	// Send the audio to the client
	err = cs.conn.WriteMessage(websocket.BinaryMessage, audioData)
	if err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

// sendStatus sends a status update to the client
func (cs *ClientState) sendStatus(status State, detail string) {
	message := StatusMessage{
		Type:   "status",
		Status: string(status),
		Detail: detail,
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling status message: %v", err)
		return
	}

	err = cs.conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

// sendTranscript sends a transcript update to the client
func (cs *ClientState) sendTranscript(text string, isFinal bool) {
	message := TranscriptMessage{
		Type:    "transcript",
		Text:    text,
		IsFinal: isFinal,
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling transcript message: %v", err)
		return
	}

	err = cs.conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

// sendResponse sends an LLM response to the client
func (cs *ClientState) sendResponse(text string) {
	message := ResponseMessage{
		Type: "response",
		Text: text,
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling response message: %v", err)
		return
	}

	err = cs.conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

// getState gets the current state thread-safely
func (cs *ClientState) getState() State {
	cs.stateMutex.Lock()
	defer cs.stateMutex.Unlock()
	return cs.state
}

// setState sets the state thread-safely
func (cs *ClientState) setState(state State) {
	cs.stateMutex.Lock()
	defer cs.stateMutex.Unlock()
	cs.state = state
}

// resetState resets the client state
func (cs *ClientState) resetState() {
	cs.setState(StateIdle)
	cs.transcript = ""
	cs.vadActive = false
	cs.triggered = false

	cs.audioBufferMutex.Lock()
	cs.audioBuffer = make([][]byte, 0)
	cs.audioBufferMutex.Unlock()

	cs.sendStatus(StateIdle, "Ready")
}

// addCancelFunc adds a cancel function thread-safely
func (cs *ClientState) addCancelFunc(key string, cancel context.CancelFunc) {
	cs.cancelMutex.Lock()
	defer cs.cancelMutex.Unlock()
	cs.cancelFuncs[key] = cancel
}

// removeCancelFunc removes a cancel function thread-safely
func (cs *ClientState) removeCancelFunc(key string) {
	cs.cancelMutex.Lock()
	defer cs.cancelMutex.Unlock()
	delete(cs.cancelFuncs, key)
}

// cancelAllOperations cancels all ongoing operations
func (cs *ClientState) cancelAllOperations() {
	cs.cancelMutex.Lock()
	defer cs.cancelMutex.Unlock()

	for _, cancel := range cs.cancelFuncs {
		cancel()
	}

	cs.cancelFuncs = make(map[string]context.CancelFunc)
}

// close closes the client state and all resources
func (cs *ClientState) close() {
	cs.closeMutex.Lock()
	defer cs.closeMutex.Unlock()

	if cs.closed {
		return
	}

	// Cancel all operations
	cs.cancelAllOperations()

	// Close the connection
	cs.conn.Close()

	cs.closed = true
}
