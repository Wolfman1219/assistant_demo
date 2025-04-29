// This is what we should have in static/audio-processor.js
class AudioProcessor extends AudioWorkletProcessor {
    constructor(options) {
        super();
        this.sampleRate = options.processorOptions?.sampleRate || 48000;
        this.targetSampleRate = options.processorOptions?.targetSampleRate || 16000;
        this.bufferSize = 4096;
        this.buffer = new Float32Array(this.bufferSize);
        this.bufferIndex = 0;
        
        // We'll need to do downsampling if the rates don't match
        this.needsResampling = this.sampleRate !== this.targetSampleRate;
        
        console.log(`AudioProcessor initialized: sampleRate=${this.sampleRate}, targetSampleRate=${this.targetSampleRate}`);
    }
    downsampleBuffer(buffer, inputSampleRate, outputSampleRate) {
        if (inputSampleRate === outputSampleRate) {
            return buffer;
        }
    
        const sampleRateRatio = inputSampleRate / outputSampleRate;
        const newLength = Math.round(buffer.length / sampleRateRatio);
        const downsampledBuffer = new Float32Array(newLength);
    
        let offset = 0;
        for (let i = 0; i < newLength; i++) {
            const nextOffset = Math.round((i + 1) * sampleRateRatio);
            let sum = 0;
            let count = 0;
    
            for (let j = offset; j < nextOffset && j < buffer.length; j++) {
                sum += buffer[j];
                count++;
            }
    
            downsampledBuffer[i] = sum / count;
            offset = nextOffset;
        }
    
        return downsampledBuffer;
    }
    process(inputs, outputs, parameters) {
        const input = inputs[0][0];
    
        if (!input) return true;
    
        for (let i = 0; i < input.length; i++) {
            this.buffer[this.bufferIndex++] = input[i];
    
            if (this.bufferIndex >= this.bufferSize) {
                let processedBuffer = this.buffer;
    
                // Downsample if needed
                if (this.needsResampling) {
                    processedBuffer = this.downsampleBuffer(
                        this.buffer,
                        this.sampleRate,
                        this.targetSampleRate
                    );
                }
    
                const pcmData = this.convertFloat32ToInt16(processedBuffer);
    
                this.port.postMessage(
                    { audioChunk: pcmData.buffer },
                    [pcmData.buffer]
                );
    
                this.buffer = new Float32Array(this.bufferSize);
                this.bufferIndex = 0;
            }
        }
    
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