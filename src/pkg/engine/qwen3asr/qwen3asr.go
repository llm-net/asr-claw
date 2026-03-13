package qwen3asr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/llm-net/asr-claw/pkg/engine"
)

func init() {
	engine.Register("qwen3-asr", func() engine.Engine { return New() })
}

// Qwen3ASR is the vLLM service engine for Qwen3-ASR (native streaming).
type Qwen3ASR struct {
	endpoint string
	model    string
}

// New creates a Qwen3ASR engine instance.
func New() *Qwen3ASR {
	endpoint := os.Getenv("QWEN3_ASR_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}
	return &Qwen3ASR{
		endpoint: endpoint,
		model:    "Qwen/Qwen3-ASR",
	}
}

func (q *Qwen3ASR) Info() engine.Capability {
	return engine.Capability{
		Name:         "qwen3-asr",
		Type:         "service",
		Languages:    []string{"zh", "en", "ja", "ko", "fr", "de", "es"},
		NeedsModel:   true,
		NeedsAPIKey:  false,
		NativeStream: true,
		Connection:   "http",
		SampleRate:   16000,
		Installed:    q.isAvailable(),
	}
}

func (q *Qwen3ASR) TranscribeFile(path string, lang string) ([]engine.Segment, error) {
	if !q.isAvailable() {
		return nil, fmt.Errorf("qwen3-asr service is not running at %s; run 'asr-claw engines start qwen3-asr'", q.endpoint)
	}

	audioData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, err
	}
	part.Write(audioData)
	writer.WriteField("model", q.model)
	writer.WriteField("language", lang)
	writer.Close()

	resp, err := http.Post(q.endpoint+"/v1/audio/transcriptions", writer.FormDataContentType(), body)
	if err != nil {
		return nil, fmt.Errorf("POST %s failed: %w", q.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("vLLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text     string `json:"text"`
		Segments []struct {
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse vLLM response: %w", err)
	}

	if len(result.Segments) > 0 {
		segments := make([]engine.Segment, len(result.Segments))
		for i, s := range result.Segments {
			segments[i] = engine.Segment{
				Index: i,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}
		return segments, nil
	}

	// Fallback: single segment from full text
	if result.Text != "" {
		return []engine.Segment{
			{Index: 0, Text: result.Text},
		}, nil
	}

	return nil, nil
}

// StreamSession creates a WebSocket streaming session.
func (q *Qwen3ASR) StreamSession(opts engine.Options) (engine.Session, error) {
	wsURL := strings.Replace(q.endpoint, "http", "ws", 1) + "/v1/audio/stream"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"X-Model":       {q.model},
		"X-Language":    {opts.Lang},
		"X-Sample-Rate": {strconv.Itoa(opts.SampleRate)},
	})
	if err != nil {
		return nil, fmt.Errorf("connect vLLM WebSocket failed: %w", err)
	}

	return &qwen3Session{conn: conn}, nil
}

func (q *Qwen3ASR) isAvailable() bool {
	resp, err := http.Get(q.endpoint + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

type qwen3Session struct {
	conn *websocket.Conn
}

func (s *qwen3Session) Feed(pcm []byte) (string, error) {
	err := s.conn.WriteMessage(websocket.BinaryMessage, pcm)
	if err != nil {
		return "", err
	}

	_, msg, err := s.conn.ReadMessage()
	if err != nil {
		return "", err
	}

	var result struct {
		Text  string `json:"text"`
		Final bool   `json:"is_final"`
	}
	json.Unmarshal(msg, &result)
	return result.Text, nil
}

func (s *qwen3Session) Finish() (string, error) {
	s.conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"finish"}`))

	_, msg, err := s.conn.ReadMessage()
	if err != nil {
		return "", err
	}

	var result struct {
		Text string `json:"text"`
	}
	json.Unmarshal(msg, &result)
	return result.Text, nil
}

func (s *qwen3Session) Close() {
	s.conn.Close()
}
