document.addEventListener('DOMContentLoaded', () => {
    // DOM Elements
    const statusIndicator = document.getElementById('status-indicator');
    const statusText = document.getElementById('status-text');
    const startBtn = document.getElementById('start-btn');
    const stopBtn = document.getElementById('stop-btn');
    const transcript = document.getElementById('transcript');
    const debugLog = document.getElementById('debug-log');

    // WebSocket and Audio Context
    let socket;
    let mediaStream;
    let audioContext;
    let sourceNode;
    let processorNode;
    let isListening = false;
    let isConnected = false;

    // Configuration
    const SAMPLE_RATE = 16000; // Must match what your VAD/STT services expect
    const BUFFER_SIZE = 4096;
    const WS_URL = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/ws`;

    // Status types and messages
    const STATUS = {
        IDLE: { class: '', text: 'Ready' },
        CONNECTING: { class: '', text: 'Connecting...' },
        LISTENING: { class: 'listening', text: 'Listening for wake word...' },
        TRIGGERED: { class: 'listening', text: 'Listening to you...' },
        PROCESSING: { class: 'thinking', text: 'Processing your request...' },
        SPEAKING: { class: 'speaking', text: 'Speaking...' },
        ERROR: { class: 'error', text: 'Error occurred' },
        DISCONNECTED: { class: '', text: 'Disconnected' }
    };

    // Set initial status
    updateStatus('IDLE');

    // Helper functions
    function log(message) {
        const timestamp = new Date().toLocaleTimeString();
        debugLog.innerHTML += `[${timestamp}] ${message}<br>`;
        debugLog.scrollTop = debugLog.scrollHeight;
        console.log(`[${timestamp}] ${message}`);
    }

    function updateStatus(statusKey, customMessage = null) {
        const status = STATUS[statusKey];
        
        // Clear all classes
        statusIndicator.className = 'status-indicator';
        
        // Add new class if specified
        if (status.class) {
            statusIndicator.classList.add(status.class);
        }
        
        // Update text
        statusText.textContent = customMessage || status.text;
        
        log(`Status: ${statusKey}${customMessage ? ` - ${customMessage}` : ''}`);
    }

    function addToTranscript(text, isUser = false) {
        const messageDiv = document.createElement('div');
        messageDiv.className = isUser ? 'user-message' : 'assistant-message';
        messageDiv.textContent = text;
        transcript.appendChild(messageDiv);
        transcript.scrollTop = transcript.scrollHeight;
    }

    // WebSocket functions
    function connectWebSocket() {
        // Close existing socket if any
        if (socket) {
            socket.close();
        }

        updateStatus('CONNECTING');
        
        socket = new WebSocket(WS_URL);
        
        socket.onopen = () => {
            isConnected = true;
            log('WebSocket connection established');
            updateStatus('IDLE');
            
            // Enable start button once connected
            startBtn.disabled = false;
        };
        
        socket.onmessage = handleWebSocketMessage;
        
        socket.onerror = (error) => {
            log(`WebSocket error: ${error}`);
            updateStatus('ERROR', 'Connection error');
        };
        
        socket.onclose = () => {
            isConnected = false;
            log('WebSocket connection closed');
            updateStatus('DISCONNECTED');
            
            // Disable buttons on disconnect
            startBtn.disabled = false;
            stopBtn.disabled = true;
            
            // Try to reconnect after a delay
            setTimeout(connectWebSocket, 3000);
        };
    }

    function handleWebSocketMessage(event) {
        // Check if the message is binary (audio) or text (status update)
        if (event.data instanceof Blob) {
            // Audio data from TTS
            processAudioResponse(event.data);
        } else {
            // Text message (status update or transcript)
            try {
                const message = JSON.parse(event.data);
                
                // Handle different message types
                switch (message.type) {
                    case 'status':
                        updateStatus(message.status, message.detail);
                        break;
                        
                    case 'transcript':
                        if (message.isFinal) {
                            addToTranscript(message.text, true);
                        }
                        break;
                        
                    case 'response':
                        addToTranscript(message.text, false);
                        break;
                        
                    default:
                        log(`Unknown message type: ${message.type}`);
                }
            } catch (error) {
                log(`Error parsing message: ${error}`);
            }
        }
    }

    // Audio processing functions
    // In the browser code that initializes the AudioContext
 // In the browser code that initializes the AudioContext
    async function initAudio() {
        try {
            // Attempt to get a mono audio stream
            mediaStream = await navigator.mediaDevices.getUserMedia({
                audio: {
                    channelCount: 1,         // Request mono
                    sampleRate: 16000,       // Request 16kHz
                }
            });
            
            // Create the AudioContext
            audioContext = new (window.AudioContext || window.webkitAudioContext)();
            
            // Create processor node
            await audioContext.audioWorklet.addModule('static/audio-processor.js');
            processorNode = new AudioWorkletNode(audioContext, 'audio-processor');
            
            // Log the audio context sample rate
            console.log(`Audio context sample rate: ${audioContext.sampleRate}Hz`);
            
            // Set up message handling
            processorNode.port.onmessage = (event) => {
                if (event.data.audioChunk && isConnected && isListening) {
                    socket.send(event.data.audioChunk);
                }
            };
            
            // Create source node from microphone
            sourceNode = audioContext.createMediaStreamSource(mediaStream);
            
            return true;
        } catch (error) {
            console.error('Error initializing audio:', error);
            return false;
        }
    }

    function startListening() {
        if (!isConnected) {
            log('Cannot start listening: Not connected');
            return;
        }
        
        if (!audioContext) {
            initAudio().then((success) => {
                if (success) startListening();
            });
            return;
        }
        
        // Connect source to processor to start capturing audio
        sourceNode.connect(processorNode);
        
        // Update state
        isListening = true;
        updateStatus('LISTENING');
        
        // Update buttons
        startBtn.disabled = true;
        stopBtn.disabled = false;
        
        log('Started listening');
    }

    function stopListening() {
        if (sourceNode && processorNode) {
            // Disconnect nodes to stop capturing audio
            sourceNode.disconnect(processorNode);
        }
        
        // Update state
        isListening = false;
        updateStatus('IDLE');
        
        // Update buttons
        startBtn.disabled = false;
        stopBtn.disabled = true;
        
        log('Stopped listening');
    }

    async function processAudioResponse(audioBlobData) {
        try {
            // Convert blob to ArrayBuffer
            const arrayBuffer = await audioBlobData.arrayBuffer();
            
            // Decode the audio data
            const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);
            
            // Create a buffer source node
            const source = audioContext.createBufferSource();
            source.buffer = audioBuffer;
            
            // Connect to destination (speakers)
            source.connect(audioContext.destination);
            
            // Play the audio
            source.start();
            
            // Log when audio ends
            source.onended = () => {
                log('TTS audio playback completed');
            };
        } catch (error) {
            log(`Error processing audio response: ${error}`);
        }
    }

    // Utility function to convert Float32Array to Int16Array
    function convertFloat32ToInt16(float32Array) {
        const int16Array = new Int16Array(float32Array.length);
        
        for (let i = 0; i < float32Array.length; i++) {
            // Convert to 16-bit signed integer
            // Clamp the value between -1 and 1, and scale to -32768 to 32767
            const s = Math.max(-1, Math.min(1, float32Array[i]));
            int16Array[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
        }
        
        return int16Array;
    }

    // Event listeners
    startBtn.addEventListener('click', startListening);
    stopBtn.addEventListener('click', stopListening);

    // Initialize connection
    connectWebSocket();
});