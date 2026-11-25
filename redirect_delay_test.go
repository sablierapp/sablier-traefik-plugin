package sablier_traefik_plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestMain turns the redirect delay off, so that the tests that go through the
// no-backend redirect do not each wait for it. The tests below set it back on
// the middleware they build.
func TestMain(m *testing.M) {
	noBackendRedirectDelay = 0
	os.Exit(m.Run())
}

// TestSablierMiddleware_NoBackendRedirectDelay covers issue #72: when Sablier
// reports the session ready before Traefik has a server to forward to, the
// blocking redirect waits, so that the client does not loop hundreds of times
// until Traefik loads the new server.
func TestSablierMiddleware_NoBackendRedirectDelay(t *testing.T) {
	const delay = 200 * time.Millisecond

	readySablier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Sablier-Session-Status", "ready")
		_, _ = w.Write([]byte("ready"))
	}))
	defer readySablier.Close()

	noServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What Traefik's load balancer answers when its pool is empty.
		http.Error(w, "no available server", http.StatusServiceUnavailable)
	})

	newMiddleware := func(t *testing.T, next http.Handler, config *Config) *SablierMiddleware {
		t.Helper()
		config.SablierURL = readySablier.URL
		config.Names = "nginx"
		config.SessionDuration = "1m"
		handler, err := New(context.Background(), next, config, "middleware")
		if err != nil {
			t.Fatal(err)
		}
		sm := handler.(*SablierMiddleware)
		sm.redirectDelay = delay
		return sm
	}

	t.Run("blocking waits before redirecting when there is no backend yet", func(t *testing.T) {
		sm := newMiddleware(t, noServer, &Config{Blocking: &BlockingConfiguration{}})

		w := httptest.NewRecorder()
		start := time.Now()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil))
		elapsed := time.Since(start)

		if w.Code != http.StatusFound {
			t.Fatalf("expected status 302, got %d (%q)", w.Code, w.Body.String())
		}
		if got := w.Header().Get("Location"); got != "/my-nginx" {
			t.Errorf("expected Location /my-nginx, got %q", got)
		}
		if elapsed < delay {
			t.Errorf("expected the redirect to wait at least %v, it took %v", delay, elapsed)
		}
	})

	t.Run("blocking returns 499 when the client goes away while waiting", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// The client goes away once the request reached Traefik's empty pool,
		// so the wait is the only thing left to interrupt.
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			noServer.ServeHTTP(w, r)
			cancel()
		})
		sm := newMiddleware(t, next, &Config{Blocking: &BlockingConfiguration{}})
		sm.redirectDelay = time.Minute

		w := httptest.NewRecorder()
		start := time.Now()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil).WithContext(ctx))

		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("expected the wait to stop with the client, it took %v", elapsed)
		}
		if w.Code != statusClientClosedRequest {
			t.Errorf("expected status %d, got %d (%q)", statusClientClosedRequest, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Location"); got != "" {
			t.Errorf("expected no redirect, got Location %q", got)
		}
	})

	t.Run("blocking does not wait when the backend answers", func(t *testing.T) {
		backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("backend"))
		})
		sm := newMiddleware(t, backend, &Config{Blocking: &BlockingConfiguration{}})
		sm.redirectDelay = time.Minute

		w := httptest.NewRecorder()
		start := time.Now()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil))

		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("expected no wait, it took %v", elapsed)
		}
		if w.Code != http.StatusOK || w.Body.String() != "backend" {
			t.Errorf("expected the backend response, got %d %q", w.Code, w.Body.String())
		}
	})

	t.Run("dynamic shows the waiting page without waiting", func(t *testing.T) {
		sm := newMiddleware(t, noServer, &Config{Dynamic: &DynamicConfiguration{}})
		sm.redirectDelay = time.Minute

		w := httptest.NewRecorder()
		start := time.Now()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil))

		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Fatalf("expected no wait, it took %v", elapsed)
		}
		if w.Code != http.StatusOK || w.Body.String() != "ready" {
			t.Errorf("expected the Sablier response, got %d %q", w.Code, w.Body.String())
		}
	})
}
