package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type data struct {
	Queried          bool `json:"lightrag_queried"`
	DecisionsChecked bool `json:"decisions_checked"`
	StatusUpdated    bool `json:"status_updated"`
}

type State struct {
	mu   sync.Mutex
	path string
	d    data
}

func New(path string) *State {
	s := &State{path: path}
	s.load()
	return s
}

func (s *State) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		return // fail open
	}
	_ = json.Unmarshal(b, &s.d)
}

func (s *State) save() {
	b, _ := json.Marshal(s.d)
	_ = os.MkdirAll(filepath.Dir(s.path), 0755)
	_ = os.WriteFile(s.path, b, 0644)
}

func (s *State) IsQueried() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d.Queried
}

func (s *State) MarkQueried() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Queried = true
	s.save()
}

func (s *State) IsDecisionsChecked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d.DecisionsChecked
}

func (s *State) MarkDecisionsChecked() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.DecisionsChecked = true
	s.save()
}

func (s *State) MarkStatusUpdated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.StatusUpdated = true
	s.save()
}
