package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Define interfaces for the service clients
// These make it easier to test and mock the services

// VadClient is the interface for the Voice Activity Detection client
type VadClient interface {
	IsActive(audioData []byte) bool
	Close() error
}

// TriggerClient is the interface for the Trigger Detection client
type TriggerClient interface {
	IsTriggered(audioData []byte) bool
	Close() error
}

// SttClient is the interface for the Speech-to-Text client
type SttClient interface {
	Transcribe(ctx context.Context, audioBuffer [][]byte) (string, error)
	Close() error
}

// LlmClient is the interface for the Language Model client
type LlmClient interface {
	GetResponse(ctx context.Context, prompt string) (chan string, error)
}

// TtsClient is the interface for the Text-to-Speech client
type TtsClient interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
	Close() error
}

// Implementation of the VAD client

type vadClientImpl struct {
	conn *grpc.ClientConn
	// Add the generated gRPC client here once you have the proto files
	// client proto.VadServiceClient
}

// NewVadClient creates a new VAD client
func NewVadClient(addr string) (VadClient, error) {
	// Connect to the gRPC server
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to VAD service: %w", err)
	}

	// Create the client
	client := &vadClientImpl{
		conn: conn,
		// Initialize the generated client
		// client: proto.NewVadServiceClient(conn),
	}

	return client, nil
}

// IsActive checks if the audio data contains voice activity
func (c *vadClientImpl) IsActive(audioData []byte) bool {
	// Mock implementation - in a real system, this would send the audio to the gRPC service
	// and get a response

	// For now, just return true if the audio has enough energy
	// This is a very naive implementation - a real VAD would be much more sophisticated
	sum := 0
	for i := 0; i < len(audioData); i += 2 {
		if i+1 < len(audioData) {
			// Convert bytes to int16
			sample := int16(audioData[i]) | (int16(audioData[i+1]) << 8)
			if sample < 0 {
				sample = -sample
			}
			sum += int(sample)
		}
	}

	// Average energy
	avg := sum / (len(audioData) / 2)

	// Threshold for activity
	threshold := 500

	return avg > threshold
}

// Close closes the VAD client
func (c *vadClientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Implementation of the Trigger client

type triggerClientImpl struct {
	conn *grpc.ClientConn
	// Add the generated gRPC client here once you have the proto files
	// client proto.TriggerServiceClient
}

// NewTriggerClient creates a new Trigger client
func NewTriggerClient(addr string) (TriggerClient, error) {
	// Connect to the gRPC server
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Trigger service: %w", err)
	}

	// Create the client
	client := &triggerClientImpl{
		conn: conn,
		// Initialize the generated client
		// client: proto.NewTriggerServiceClient(conn),
	}

	return client, nil
}

// IsTriggered checks if the audio data contains the trigger word
func (c *triggerClientImpl) IsTriggered(audioData []byte) bool {
	// Mock implementation - in a real system, this would send the audio to the gRPC service
	// and get a response

	// For demo purposes, just randomly return true occasionally
	return time.Now().Unix()%10 == 0
}

// Close closes the Trigger client
func (c *triggerClientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Implementation of the STT client

type sttClientImpl struct {
	conn *grpc.ClientConn
	// Add the generated gRPC client here once you have the proto files
	// client proto.SttServiceClient
}

// NewSttClient creates a new STT client
func NewSttClient(addr string) (SttClient, error) {
	// Connect to the gRPC server
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to STT service: %w", err)
	}

	// Create the client
	client := &sttClientImpl{
		conn: conn,
		// Initialize the generated client
		// client: proto.NewSttServiceClient(conn),
	}

	return client, nil
}

// Transcribe transcribes the audio data
func (c *sttClientImpl) Transcribe(ctx context.Context, audioBuffer [][]byte) (string, error) {
	// Mock implementation - in a real system, this would send the audio to the gRPC service
	// and get a response

	// For demo purposes, just return a fixed transcript
	return "Hello, how can I help you today?", nil
}

// Close closes the STT client
func (c *sttClientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Implementation of the LLM client

type llmClientImpl struct {
	baseURL string
	client  *http.Client
}

// NewLlmClient creates a new LLM client
func NewLlmClient(baseURL string) LlmClient {
	return &llmClientImpl{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// LLMRequest represents a request to the LLM service
type LLMRequest struct {
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// LLMResponse represents a response from the LLM service
type LLMResponse struct {
	Response string `json:"response"`
}

// GetResponse gets a response from the LLM service
func (c *llmClientImpl) GetResponse(ctx context.Context, prompt string) (chan string, error) {
	// Create a channel to stream the response
	responseChan := make(chan string)

	// Create the request
	reqBody, err := json.Marshal(LLMRequest{
		Prompt: prompt,
		Stream: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal LLM request: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Start a goroutine to handle the response
	go func() {
		defer close(responseChan)

		// Mock implementation - in a real system, this would send the request to the HTTP service
		// and stream the response

		// For demo purposes, just send a fixed response character by character
		response := "I'm your AI assistant. I can help you with various tasks, answer questions, and provide information on a wide range of topics. Just let me know what you need!"

		for _, char := range response {
			select {
			case <-ctx.Done():
				return
			case responseChan <- string(char):
				time.Sleep(50 * time.Millisecond) // Simulate streaming delay
			}
		}
	}()

	return responseChan, nil
}

// Implementation of the TTS client

type ttsClientImpl struct {
	conn *grpc.ClientConn
	// Add the generated gRPC client here once you have the proto files
	// client proto.TtsServiceClient
}

// NewTtsClient creates a new TTS client
func NewTtsClient(addr string) (TtsClient, error) {
	// Connect to the gRPC server
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to TTS service: %w", err)
	}

	// Create the client
	client := &ttsClientImpl{
		conn: conn,
		// Initialize the generated client
		// client: proto.NewTtsServiceClient(conn),
	}

	return client, nil
}

// Synthesize synthesizes text to speech
func (c *ttsClientImpl) Synthesize(ctx context.Context, text string) ([]byte, error) {
	// Mock implementation - in a real system, this would send the text to the gRPC service
	// and get audio data response

	// For demo purposes, just return an empty audio buffer
	// In a real implementation, this would return WAV or MP3 audio data
	return []byte{}, nil
}

// Close closes the TTS client
func (c *ttsClientImpl) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
