package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/llm-net/asr-claw/pkg/engine"
)

func init() {
	engine.Register("openai", func() engine.Engine { return New() })
}

// OpenAI is the OpenAI Whisper API cloud engine.
type OpenAI struct {
	apiKey  string
	model   string
	baseURL string
}

// New creates an OpenAI engine instance.
func New() *OpenAI {
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := os.Getenv("OPENAI_ASR_MODEL")
	if model == "" {
		model = "whisper-1"
	}
	return &OpenAI{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		model:   model,
		baseURL: baseURL,
	}
}

func (o *OpenAI) Info() engine.Capability {
	return engine.Capability{
		Name:         "openai",
		Type:         "cloud",
		Languages:    []string{"zh", "en", "ja", "ko", "fr", "de", "es", "pt", "ru", "ar"},
		NeedsModel:   false,
		NeedsAPIKey:  true,
		NativeStream: false,
		Connection:   "https",
		SampleRate:   16000,
		Installed:    o.apiKey != "",
	}
}

func (o *OpenAI) TranscribeFile(path string, lang string) ([]engine.Segment, error) {
	if o.apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set; export OPENAI_API_KEY=your-key")
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
	writer.WriteField("model", o.model)
	writer.WriteField("language", lang)
	writer.WriteField("response_format", "verbose_json")
	writer.Close()

	url := o.baseURL + "/audio/transcriptions"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Text     string `json:"text"`
		Segments []struct {
			ID    int     `json:"id"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
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

	if result.Text != "" {
		return []engine.Segment{
			{Index: 0, Text: result.Text},
		}, nil
	}

	return nil, nil
}
