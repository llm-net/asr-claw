package engine

import (
	"fmt"
	"sort"
)

var engines = map[string]func() Engine{}

// Register adds an engine factory to the registry.
func Register(name string, factory func() Engine) {
	engines[name] = factory
}

// Get returns an engine instance by name.
func Get(name string) (Engine, error) {
	factory, ok := engines[name]
	if !ok {
		return nil, fmt.Errorf("unknown engine: %s", name)
	}
	return factory(), nil
}

// List returns capabilities of all registered engines.
func List() []Capability {
	var caps []Capability
	for _, factory := range engines {
		e := factory()
		caps = append(caps, e.Info())
	}
	sort.Slice(caps, func(i, j int) bool {
		return caps[i].Name < caps[j].Name
	})
	return caps
}

// Names returns sorted list of all registered engine names.
func Names() []string {
	var names []string
	for name := range engines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AutoSelect picks the best available engine based on priority:
// 1. Running service engines (qwen3-asr)
// 2. Installed CLI engines (qwen-asr > whisper)
// 3. Configured cloud engines (doubao > openai > deepgram)
func AutoSelect() (Engine, error) {
	priority := []string{
		"qwen3-asr",     // service, native stream (vLLM GPU)
		"qwen-asr",      // cli, local (antirez/qwen-asr, Mac/Linux)
		"whisper",        // cli, local (whisper.cpp)
		"doubao",         // cloud
		"openai",         // cloud
		"deepgram",       // cloud
	}

	for _, name := range priority {
		factory, ok := engines[name]
		if !ok {
			continue
		}
		e := factory()
		cap := e.Info()
		if cap.Installed {
			return e, nil
		}
	}

	return nil, fmt.Errorf("no available engine found; run 'asr-claw engines list' to see options, or 'asr-claw engines install <engine>' to install one")
}
