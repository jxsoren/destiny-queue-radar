//go:build !e2e

package web

import (
	"net/http"
	"testing"
)

func TestE2ELogin_NotInNormalBuilds(t *testing.T) {
	for name, e := range map[string]*testEnv{"placeholder": newTestEnv(t), "static": newStaticEnv(t)} {
		if rec := e.do(t, http.MethodGet, "/auth/e2e-login", nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: /auth/e2e-login = %d, want 404", name, rec.Code)
		}
	}
}
