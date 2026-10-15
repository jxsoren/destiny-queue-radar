package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

type indexLine struct {
	Time    time.Time `json:"time"`
	Label   string    `json:"label"`
	File    string    `json:"file"`
	Summary Summary   `json:"summary"`
}

// Recorder saves raw profile responses plus an index.jsonl describing each one.
type Recorder struct {
	dir   string
	mu    sync.Mutex
	index *os.File
}

func NewRecorder(dir string) (*Recorder, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, "index.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	return &Recorder{dir: dir, index: f}, nil
}

func (r *Recorder) Record(at time.Time, label string, s Summary, raw []byte) error {
	rel := filepath.Join(slug(s.Player), at.UTC().Format("20060102T150405.000Z")+".json")
	full := filepath.Join(r.dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("create player dir: %w", err)
	}
	if err := os.WriteFile(full, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	line, err := json.Marshal(indexLine{Time: at, Label: label, File: rel, Summary: s})
	if err != nil {
		return fmt.Errorf("marshal index line: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.index.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append index: %w", err)
	}
	return nil
}

func (r *Recorder) Close() error { return r.index.Close() }

func slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// Label is the operator's current description of what the probed players are doing.
type Label struct {
	mu sync.Mutex
	v  string
}

func (l *Label) Set(v string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.v = strings.TrimSpace(v)
}

func (l *Label) Get() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.v
}
