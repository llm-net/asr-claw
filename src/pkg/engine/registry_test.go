package engine

import (
	"testing"
)

func TestRegisterAndGet(t *testing.T) {
	// Save and restore original engines
	orig := engines
	engines = map[string]func() Engine{}
	defer func() { engines = orig }()

	Register("test-engine", func() Engine {
		return &mockEngine{cap: Capability{Name: "test-engine", Type: "cli", Installed: true}}
	})

	eng, err := Get("test-engine")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if eng.Info().Name != "test-engine" {
		t.Errorf("name: got %q, want %q", eng.Info().Name, "test-engine")
	}
}

func TestGet_Unknown(t *testing.T) {
	orig := engines
	engines = map[string]func() Engine{}
	defer func() { engines = orig }()

	_, err := Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown engine")
	}
}

func TestList(t *testing.T) {
	orig := engines
	engines = map[string]func() Engine{}
	defer func() { engines = orig }()

	Register("b-engine", func() Engine {
		return &mockEngine{cap: Capability{Name: "b-engine"}}
	})
	Register("a-engine", func() Engine {
		return &mockEngine{cap: Capability{Name: "a-engine"}}
	})

	caps := List()
	if len(caps) != 2 {
		t.Fatalf("expected 2 engines, got %d", len(caps))
	}
	// Should be sorted
	if caps[0].Name != "a-engine" {
		t.Errorf("first engine: got %q, want %q", caps[0].Name, "a-engine")
	}
}

func TestAutoSelect_NoEngines(t *testing.T) {
	orig := engines
	engines = map[string]func() Engine{}
	defer func() { engines = orig }()

	_, err := AutoSelect()
	if err == nil {
		t.Error("expected error when no engines available")
	}
}

func TestAutoSelect_PicksInstalled(t *testing.T) {
	orig := engines
	engines = map[string]func() Engine{}
	defer func() { engines = orig }()

	Register("whisper", func() Engine {
		return &mockEngine{cap: Capability{Name: "whisper", Installed: true}}
	})
	Register("openai", func() Engine {
		return &mockEngine{cap: Capability{Name: "openai", Installed: false}}
	})

	eng, err := AutoSelect()
	if err != nil {
		t.Fatalf("AutoSelect error: %v", err)
	}
	if eng.Info().Name != "whisper" {
		t.Errorf("selected: got %q, want %q", eng.Info().Name, "whisper")
	}
}

type mockEngine struct {
	cap Capability
}

func (m *mockEngine) Info() Capability {
	return m.cap
}

func (m *mockEngine) TranscribeFile(path string, lang string) ([]Segment, error) {
	return []Segment{{Index: 0, Text: "mock text"}}, nil
}
