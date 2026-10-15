package probe

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorder_WritesRawFileAndIndexLine(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(dir)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 15, 18, 5, 7, 123_000_000, time.UTC)
	s := Summary{Player: "Clae™#0880", Activity: "Orbit"}
	if err := r.Record(at, "P1 orbit", s, []byte(`{"raw":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}

	rel := filepath.Join("Clae__0880", "20261015T180507.123Z.json")
	raw, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil || string(raw) != `{"raw":true}` {
		t.Fatalf("raw file: %q, %v", raw, err)
	}

	f, err := os.Open(filepath.Join(dir, "index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("index.jsonl is empty")
	}
	var line indexLine
	if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
		t.Fatal(err)
	}
	if line.Label != "P1 orbit" || line.File != rel || line.Summary.Activity != "Orbit" || !line.Time.Equal(at) {
		t.Errorf("line = %+v", line)
	}
}

func TestRecorder_AppendsAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		r, err := NewRecorder(dir)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 10, 15, 18, 0, i, 0, time.UTC)
		if err := r.Record(at, "", Summary{Player: "A#0001"}, []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		r.Close()
	}
	b, err := os.ReadFile(filepath.Join(dir, "index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(b, []byte("\n")); n != 2 {
		t.Errorf("index lines = %d, want 2", n)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Kwivor#1993":        "Kwivor_1993",
		"Clae™#0880":         "Clae__0880",
		"Badluck Apolo#7173": "Badluck_Apolo_7173",
		"":                   "unknown",
		"../x":               "___x",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	var l Label
	if l.Get() != "" {
		t.Error("zero label should be empty")
	}
	l.Set("  P1 in comp  \n")
	if l.Get() != "P1 in comp" {
		t.Errorf("label = %q", l.Get())
	}
}
