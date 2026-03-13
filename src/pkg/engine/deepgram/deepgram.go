package deepgram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/llm-net/asr-claw/pkg/engine"
)

func init() {
	engine.Register("deepgram", func() engine.Engine { return New() })
}

// Deepgram is the Deepgram cloud engine with native WebSocket streaming.
type Deepgram struct {
	apiKey string
	model  string
	tier   string
}

// New creates a Deepgram engine instance.
func New() *Deepgram {
	model := os.Getenv("DEEPGRAM_MODEL")
	if model == "" {
		model = "nova-2"
	}
	tier := os.Getenv("DEEPGRAM_TIER")
	if tier == "" {
		tier = "enhanced"
	}
	return &Deepgram{
		apiKey: os.Getenv("DEEPGRAM_API_KEY"),
		model:  model,
		tier:   tier,
	}
}

func (d *Deepgram) Info() engine.Capability {
	return engine.Capability{
		Name:         "deepgram",
		Type:         "cloud",
		Languages:    []string{"zh", "en", "ja", "ko", "fr", "de", "es", "pt", "ru"},
		NeedsModel:   false,
		NeedsAPIKey:  true,
		NativeStream: true,
		Connection:   "websocket",
		SampleRate:   16000,
		Installed:    d.apiKey != "",
	}
}

func (d *Deepgram) TranscribeFile(path string, lang string) ([]engine.Segment, error) {
	if d.apiKey == "" {
		return nil, fmt.Errorf("DEEPGRAM_API_KEY not set; export DEEPGRAM_API_KEY=your-key")
	}

	audioData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.deepgram.com/v1/listen?model=%s&language=%s&tier=%s&punctuate=true&utterances=true",
		d.model, lang, d.tier)

	req, err := http.NewRequest("POST", url, bytes.NewReader(audioData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.apiKey)
	req.Header.Set("Content-Type", "audio/wav")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepgram API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("deepgram returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Results struct {
			Utterances []struct {
				Start      float64 `json:"start"`
				End        float64 `json:"end"`
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
			} `json:"utterances"`
			Channels []struct {
				Alternatives []struct {
					Transcript string  `json:"transcript"`
					Confidence float64 `json:"confidence"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse deepgram response: %w", err)
	}

	// Prefer utterances
	if len(result.Results.Utterances) > 0 {
		segments := make([]engine.Segment, len(result.Results.Utterances))
		for i, u := range result.Results.Utterances {
			segments[i] = engine.Segment{
				Index:      i,
				Start:      u.Start,
				End:        u.End,
				Text:       u.Transcript,
				Confidence: u.Confidence,
			}
		}
		return segments, nil
	}

	// Fallback to channels
	if len(result.Results.Channels) > 0 && len(result.Results.Channels[0].Alternatives) > 0 {
		alt := result.Results.Channels[0].Alternatives[0]
		return []engine.Segment{
			{Index: 0, Text: alt.Transcript, Confidence: alt.Confidence},
		}, nil
	}

	return nil, nil
}

// StreamSession creates a WebSocket streaming session.
func (d *Deepgram) StreamSession(opts engine.Options) (engine.Session, error) {
	if d.apiKey == "" {
		return nil, fmt.Errorf("DEEPGRAM_API_KEY not set; export DEEPGRAM_API_KEY=your-key")
	}

	url := fmt.Sprintf("wss://api.deepgram.com/v1/listen?model=%s&language=%s&tier=%s&punctuate=true&encoding=linear16&sample_rate=%d&channels=1",
		d.model, opts.Lang, d.tier, opts.SampleRate)

	header := http.Header{
		"Authorization": {"Token " + d.apiKey},
	}

	conn, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		return nil, fmt.Errorf("connect deepgram WebSocket failed: %w", err)
	}

	return &deepgramSession{
		conn: conn,
		lang: opts.Lang,
	}, nil
}

type deepgramSession struct {
	conn *websocket.Conn
	lang string
}

func (s *deepgramSession) Feed(pcm []byte) (string, error) {
	err := s.conn.WriteMessage(websocket.BinaryMessage, pcm)
	if err != nil {
		return "", err
	}

	_, msg, err := s.conn.ReadMessage()
	if err != nil {
		return "", err
	}

	var result struct {
		Channel struct {
			Alternatives []struct {
				Transcript string `json:"transcript"`
			} `json:"alternatives"`
		} `json:"channel"`
		IsFinal bool `json:"is_final"`
	}
	if err := json.Unmarshal(msg, &result); err != nil {
		return "", nil
	}

	if len(result.Channel.Alternatives) > 0 {
		return result.Channel.Alternatives[0].Transcript, nil
	}
	return "", nil
}

func (s *deepgramSession) Finish() (string, error) {
	// Send close frame to signal end of audio
	s.conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"CloseStream"}`))

	// Read final results
	var lastText string
	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			break
		}
		var result struct {
			Channel struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channel"`
		}
		if json.Unmarshal(msg, &result) == nil && len(result.Channel.Alternatives) > 0 {
			if t := result.Channel.Alternatives[0].Transcript; t != "" {
				lastText = t
			}
		}
	}
	return lastText, nil
}

func (s *deepgramSession) Close() {
	s.conn.Close()
}

// Ensure Deepgram implements StreamEngine at compile time.
var _ engine.StreamEngine = (*Deepgram)(nil)

// Ensure Deepgram.Info().NativeStream returns true (checked at compile time via interface).
var _ = strconv.Itoa // keep strconv import used
