package service

import (
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

var templateRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9_.]+)\}\}`)

type State struct {
	mu   sync.RWMutex
	data map[string]any
}

func newState() *State {
	return &State{
		data: make(map[string]any),
	}
}

func (s *State) Set(stepID string, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[stepID] = result
}

func (s *State) Get(stepID string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[stepID]
	return val, ok
}

func (s *State) Resolve(params map[string]any) map[string]any {
	resolved := make(map[string]any)

	for k, v := range params {
		if strVal, ok := v.(string); ok {
			resolved[k] = templateRegex.ReplaceAllStringFunc(strVal, func(str string) string {
				path := strings.Trim(str, "{}")
				parts := strings.Split(path, ".")

				if len(parts) > 0 {
					s.mu.RLock()
					stepData, exists := s.data[parts[0]]
					s.mu.RUnlock()

					if exists {
						val := navigatePath(stepData, parts[1:])
						if valStr, ok := val.(string); ok {
							return valStr
						}

						b, _ := json.Marshal(val)
						return string(b)
					}
				}

				return str
			})
		} else {
			resolved[k] = v
		}
	}
	return resolved
}

func (s *State) Copy() *State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := make(map[string]any, len(s.data))
	for k, v := range s.data {
		data[k] = v
	}
	return &State{data: data}
}

func navigatePath(data any, path []string) any {
	current := data
	for _, p := range path {
		if currentMap, ok := current.(map[string]any); ok {
			current = currentMap[p]
		} else {
			return nil
		}
	}

	return current
}
