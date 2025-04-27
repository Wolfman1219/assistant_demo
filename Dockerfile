# Build stage
FROM golang:1.18-alpine AS builder

# Set working directory
WORKDIR /app

# Install necessary tools
RUN apk add --no-cache make git protoc protobuf-dev

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy frontend files to static directory
RUN mkdir -p static
COPY index.html style.css app.js audio-processor.js ./static/

# Build the application
RUN go build -o ai-assistant *.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/ai-assistant .

# Copy static files
COPY --from=builder /app/static ./static

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV VAD_SERVICE=vad-service:50051
ENV TRIGGER_SERVICE=trigger-service:50052
ENV STT_SERVICE=stt-service:50053
ENV TTS_SERVICE=tts-service:50054
ENV LLM_SERVICE=http://llm-service:8000

# Run the application
CMD ["./ai-assistant"]