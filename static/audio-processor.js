// audio-processor.js
class AudioProcessor extends AudioWorkletProcessor {
    constructor(options) {
        super();
        // Get sample rates from options or use defaults
        this.sampleRate = options.processorOptions?.sampleRate || 48000;
        this.targetSampleRate = options.processorOptions?.targetSampleRate || 16000;
        
        // Fixed buffer size that VAD expects
        this.targetBufferSize = 512; // This is what Silero VAD expects
        
        // Create buffer to hold audio samples
        this.buffer = new Float32Array(this.targetBufferSize);
        this.bufferIndex = 0;
        
        // Calculate resampling ratio
        this.resampleRatio = this.targetSampleRate / this.sampleRate;
        this.needsResampling = this.sampleRate !== this.targetSampleRate;
        
        // Track our position for resampling
        this.resamplePos = 0;
        
        console.log(`AudioProcessor initialized: sampleRate=${this.sampleRate}, targetSampleRate=${this.targetSampleRate}, ratio=${this.resampleRatio}`);
    }

    process(inputs, outputs, parameters) {
        // Get the input data from channel 0
        const input = inputs[0]?.[0];

        if (!input || input.length === 0) return true;

        if (this.needsResampling) {
            // Perform simple linear resampling
            while (this.resamplePos < input.length && this.bufferIndex < this.targetBufferSize) {
                // Get the current position in the input (might be fractional)
                const inputPos = Math.floor(this.resamplePos);
                const fraction = this.resamplePos - inputPos;
                
                // Get the current and next sample (if available)
                const currentSample = input[inputPos];
                const nextSample = (inputPos + 1 < input.length) ? input[inputPos + 1] : currentSample;
                
                // Linear interpolation between samples
                const interpolatedSample = currentSample + fraction * (nextSample - currentSample);
                
                // Add to buffer
                this.buffer[this.bufferIndex++] = interpolatedSample;
                
                // Move position according to ratio
                this.resamplePos += 1 / this.resampleRatio;
                
                // If we've reached the end of the input, reset position
                if (this.resamplePos >= input.length) {
                    this.resamplePos = this.resamplePos - input.length;
                }
                
                // If buffer is full, send it
                if (this.bufferIndex >= this.targetBufferSize) {
                    this.sendBuffer();
                }
            }
        } else {
            // No resampling needed, just copy directly
            for (let i = 0; i < input.length; i++) {
                this.buffer[this.bufferIndex++] = input[i];
                
                // If buffer is full, send it
                if (this.bufferIndex >= this.targetBufferSize) {
                    this.sendBuffer();
                }
            }
        }

        // Return true to keep the processor alive
        return true;
    }

    sendBuffer() {
        // Only send complete buffers
        if (this.bufferIndex < this.targetBufferSize) {
            return;
        }
        
        // Convert the float audio data to 16-bit PCM
        const int16Array = new Int16Array(this.targetBufferSize);
        for (let i = 0; i < this.targetBufferSize; i++) {
            // Clamp the value between -1 and 1, and scale to -32768 to 32767
            const sample = Math.max(-1, Math.min(1, this.buffer[i]));
            int16Array[i] = sample < 0 ? sample * 0x8000 : sample * 0x7FFF;
        }
        
        // Send the audio chunk to the main thread
        this.port.postMessage(int16Array, [int16Array.buffer]);
        
        // Reset the buffer
        this.bufferIndex = 0;
    }
}

// Register the processor
registerProcessor('audio-processor', AudioProcessor);