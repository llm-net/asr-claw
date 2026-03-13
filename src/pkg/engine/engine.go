package engine

// Engine is the base interface for all ASR engines.
// CLI engines (whisper.cpp, qwen3-asr-rs) implement this interface.
type Engine interface {
	// Info returns engine capabilities.
	Info() Capability

	// TranscribeFile transcribes a single audio file.
	// path: WAV file path (16kHz, 16bit, mono)
	// lang: language code (zh, en, ja, etc.)
	TranscribeFile(path string, lang string) ([]Segment, error)
}

// StreamEngine extends Engine with native streaming support.
// Service engines (qwen3-asr vLLM, deepgram) implement this interface.
type StreamEngine interface {
	Engine

	// StreamSession creates a streaming transcription session.
	StreamSession(opts Options) (Session, error)
}

// Session represents an active streaming transcription session.
type Session interface {
	// Feed sends a chunk of PCM data, returns current recognized text.
	Feed(pcm []byte) (text string, err error)

	// Finish ends streaming input, returns final confirmed text.
	Finish() (text string, err error)

	// Close releases session resources.
	Close()
}

// Capability describes an engine's capabilities and requirements.
type Capability struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`          // "cli" | "service" | "cloud"
	Languages    []string `json:"languages"`
	NeedsModel   bool     `json:"needs_model"`
	NeedsAPIKey  bool     `json:"needs_api_key"`
	NativeStream bool     `json:"native_stream"`
	Connection   string   `json:"connection"`    // "subprocess" | "http" | "websocket"
	SampleRate   int      `json:"sample_rate"`
	Installed    bool     `json:"installed"`
}

// Segment represents a transcription result segment.
type Segment struct {
	Index      int     `json:"index"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Text       string  `json:"text"`
	Lang       string  `json:"lang,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// Options for streaming session creation.
type Options struct {
	Lang          string `json:"lang"`
	SampleRate    int    `json:"sample_rate"`
	Channels      int    `json:"channels"`
	BitsPerSample int    `json:"bits"`
}
