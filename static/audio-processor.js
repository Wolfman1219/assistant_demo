// audio-processor.js
// This is what we should have in static/audio-processor.js
class AudioProcessor extends AudioWorkletProcessor {
    constructor(options) {
        super();
        this.sampleRate = options.processorOptions?.sampleRate || 48000;
        this.targetSampleRate = options.processorOptions?.targetSampleRate || 16000;
        this.bufferSize = 2048;
        this.buffer = new Float32Array(this.bufferSize);
        this.bufferIndex = 0;
        
        // We'll need to do downsampling if the rates don't match
        this.needsResampling = this.sampleRate !== this.targetSampleRate;
        
        console.log(`AudioProcessor initialized: sampleRate=${this.sampleRate}, targetSampleRate=${this.targetSampleRate}`);
    }

    process(inputs, outputs, parameters) {
        // Get the input data from channel 0 (mono)
        const input = inputs[0][0];

        if (!input) return true;

        // Copy input data to our buffer
        for (let i = 0; i < input.length; i++) {
            this.buffer[this.bufferIndex++] = input[i];

            // If the buffer is full, send it and reset
            if (this.bufferIndex >= this.bufferSize) {
                // Convert the float audio data to 16-bit PCM
                const pcmData = this.convertFloat32ToInt16(this.buffer);
                
                // Send the audio chunk to the main thread
                this.port.postMessage({
                    audioChunk: pcmData.buffer
                }, [pcmData.buffer]); // Transfer ownership for better performance
                
                // Reset the buffer
                this.buffer = new Float32Array(this.bufferSize);
                this.bufferIndex = 0;
            }
        }

        // Return true to keep the processor alive
        return true;
    }

    convertFloat32ToInt16(float32Array) {
        const int16Array = new Int16Array(float32Array.length);
        
        for (let i = 0; i < float32Array.length; i++) {
            // Clamp the value between -1 and 1, and scale to -32768 to 32767
            const s = Math.max(-1, Math.min(1, float32Array[i]));
            int16Array[i] = s < 0 ? s * 0x8000 : s * 0x7FFF;
        }
        
        return int16Array;
    }
}

// Register the processor
registerProcessor('audio-processor', AudioProcessor);