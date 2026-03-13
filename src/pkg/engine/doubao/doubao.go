package doubao

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
	engine.Register("doubao", func() engine.Engine { return New() })
}

// Doubao is the Volcengine Seed-ASR 2.0 cloud engine.
type Doubao struct {
	apiKey  string
	appID   string
	cluster string
}

// New creates a Doubao engine instance.
func New() *Doubao {
	return &Doubao{
		apiKey:  os.Getenv("DOUBAO_API_KEY"),
		appID:   os.Getenv("DOUBAO_APP_ID"),
		cluster: os.Getenv("DOUBAO_CLUSTER"),
	}
}

func (d *Doubao) Info() engine.Capability {
	return engine.Capability{
		Name:         "doubao",
		Type:         "cloud",
		Languages:    []string{"zh", "en", "ja", "ko"},
		NeedsModel:   false,
		NeedsAPIKey:  true,
		NativeStream: false,
		Connection:   "https",
		SampleRate:   16000,
		Installed:    d.apiKey != "",
	}
}

func (d *Doubao) TranscribeFile(path string, lang string) ([]engine.Segment, error) {
	if d.apiKey == "" {
		return nil, fmt.Errorf("DOUBAO_API_KEY not set; export DOUBAO_API_KEY=your-key")
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
	writer.WriteField("language", lang)
	if d.appID != "" {
		writer.WriteField("app_id", d.appID)
	}
	if d.cluster != "" {
		writer.WriteField("cluster", d.cluster)
	}
	writer.Close()

	url := "https://openspeech.bytedance.com/api/v1/auc/submit"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+d.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doubao API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("doubao returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Utterances []struct {
				Text      string  `json:"text"`
				StartTime float64 `json:"start_time"`
				EndTime   float64 `json:"end_time"`
			} `json:"utterances"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse doubao response: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("doubao error %d: %s", result.Code, result.Message)
	}

	segments := make([]engine.Segment, len(result.Data.Utterances))
	for i, u := range result.Data.Utterances {
		segments[i] = engine.Segment{
			Index: i,
			Start: u.StartTime / 1000.0,
			End:   u.EndTime / 1000.0,
			Text:  u.Text,
		}
	}
	return segments, nil
}
