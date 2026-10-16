package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type noteBody struct {
	Note string `json:"note"`
}

func TestDecodeJSON_RejectsTrailingData(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"note":"a"}{"note":"b"}`))
	var v noteBody
	if err := decodeJSON(httptest.NewRecorder(), req, &v); err == nil {
		t.Error("trailing JSON value accepted")
	}
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"note":"a"}`+"\n"))
	if err := decodeJSON(httptest.NewRecorder(), req, &v); err != nil {
		t.Errorf("trailing whitespace rejected: %v", err)
	}
}

func TestBadBody_MapsStatus(t *testing.T) {
	big := `{"note":"` + strings.Repeat("x", maxBodyBytes+10) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	rec := httptest.NewRecorder()
	err := decodeJSON(rec, req, &noteBody{})
	badBody(rec, err)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body = %d, want 413", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"bogus":1}`))
	rec = httptest.NewRecorder()
	badBody(rec, decodeJSON(rec, req, &noteBody{}))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", rec.Code)
	}
}

func TestWriteJSON_SetsNoSniff(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"a": "b"})
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
}
