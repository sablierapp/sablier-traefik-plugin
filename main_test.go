package sablier_traefik_plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSablierMiddleware_ServeHTTP(t *testing.T) {
	type fields struct {
		Next    http.Handler
		Config  *Config
		Headers *map[string]string
	}
	type sablier struct {
		headers map[string]string
		body    string
		code    int
	}
	tests := []struct {
		name     string
		fields   fields
		sablier  sablier
		expected string
		method   string
		code     int
	}{
		{
			name: "sablier service is ready",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")

				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Dynamic:         &DynamicConfiguration{},
				},
			},
			expected: "response from service",
			code:     200,
		},
		{
			name: "sablier service is not ready",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "not-ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Dynamic:         &DynamicConfiguration{},
				},
			},
			expected: "response from sablier",
			code:     200,
		},
		{
			name: "sablier service is ready but 503",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Dynamic:         &DynamicConfiguration{},
				},
			},
			expected: "response from sablier",
			code:     200,
		},
		{
			name: "sablier service is ready blocking",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Blocking:        &BlockingConfiguration{},
				},
			},
			expected: "response from service",
			code:     200,
		},
		{
			name: "sablier service is not ready blocking",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "not-ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Blocking:        &BlockingConfiguration{},
				},
			},
			expected: "response from sablier",
			// is this correct for blocking? I would expect to get error
			code: 200,
		},
		{
			name: "sablier service is ready blocking but 503",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Blocking:        &BlockingConfiguration{},
				},
			},
			expected: "Found",
			code:     302,
		},
		{
			name: "sablier service ignores request from configured user agent",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "not-ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")

				}),
				Config: &Config{
					Names:            "nginx",
					SessionDuration:  "1m",
					Dynamic:          &DynamicConfiguration{},
					IgnoreUserAgent: []string{"curl"},
				},
				Headers: &map[string]string{
					"User-Agent": "curl/8.7.1",
				},
			},
			expected: "request with user agent ignored as configured",
			code:     200,
		},
		{
			name: "sablier service is ready when non ignored user agent requests",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")

				}),
				Config: &Config{
					Names:            "nginx",
					SessionDuration:  "1m",
					Dynamic:          &DynamicConfiguration{},
					IgnoreUserAgent: []string{"curl"},
				},
				Headers: &map[string]string{
					"User-Agent": "Mozilla",
				},
			},
			expected: "response from service",
			code:     200,
		},
		{
			name: "sablier response non-200 status code is forwarded when not ready",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "not-ready",
				},
				body: "loading page",
				code: http.StatusAccepted,
			},
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					httptrace.ContextClientTrace(r.Context()).WroteHeaders()
					_, _ = fmt.Fprint(w, "response from service")
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Dynamic:         &DynamicConfiguration{},
				},
			},
			expected: "loading page",
			code:     http.StatusAccepted,
		},
		{
			name: "blocking mode POST returns 307 when session ready but backend 503",
			sablier: sablier{
				headers: map[string]string{
					"X-Sablier-Session-Status": "ready",
				},
				body: "response from sablier",
			},
			method: http.MethodPost,
			fields: fields{
				Next: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// No httptrace callback — simulates Traefik 503 (no backend in pool),
					// so conditonalResponseWriter.ready stays false.
					w.WriteHeader(http.StatusServiceUnavailable)
				}),
				Config: &Config{
					Names:           "nginx",
					SessionDuration: "1m",
					Blocking:        &BlockingConfiguration{},
				},
			},
			expected: "Temporary Redirect",
			code:     http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.sablier.headers {
					w.Header().Add(key, value)
				}
				if tt.sablier.code != 0 {
					w.WriteHeader(tt.sablier.code)
				}
				_, err := w.Write([]byte(tt.sablier.body))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}))
			defer sablierMockServer.Close()

			tt.fields.Config.SablierURL = sablierMockServer.URL

			sm, err := New(context.Background(), tt.fields.Next, tt.fields.Config, "middleware")
			if err != nil {
				panic(err)
			}

			method := http.MethodGet
			if tt.method != "" {
				method = tt.method
			}
			req := httptest.NewRequest(method, "/my-nginx", nil)
			w := httptest.NewRecorder()

			if tt.fields.Headers != nil {
				for k, v := range *tt.fields.Headers {
					req.Header.Add(k, v)
				}
			}

			sm.ServeHTTP(w, req)

			res := w.Result()
			defer func() {
				_ = res.Body.Close() //nolint:errcheck
			}()
			data, err := io.ReadAll(res.Body)
			if err != nil {
				t.Errorf("expected error to be nil got %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("expected '%s' got '%v'", tt.expected, string(data))
			}
			if res.StatusCode != tt.code {
				t.Errorf("expected '%d' got '%d'", tt.code, res.StatusCode)

			}
		})
	}
}

func TestSablierMiddleware_ServeHTTP_FailOpen(t *testing.T) {
	closedSablier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	sablierURL := closedSablier.URL
	closedSablier.Close()

	t.Run("passes through to backend when Sablier is unreachable and failOpen is enabled", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Backend", "reached")
			w.WriteHeader(http.StatusAccepted)
			_, _ = fmt.Fprint(w, "response from service")
		})

		sm, err := New(context.Background(), next, &Config{
			SablierURL:      sablierURL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
			FailOpen:        true,
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil))

		res := w.Result()
		defer res.Body.Close() //nolint:errcheck

		if res.StatusCode != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, res.StatusCode)
		}
		if got := res.Header.Get("X-Backend"); got != "reached" {
			t.Errorf("expected backend header to be preserved, got %q", got)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "response from service" {
			t.Errorf("expected backend body, got %q", body)
		}
	})

	t.Run("returns 500 when Sablier is unreachable and failOpen is disabled", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		})

		sm, err := New(context.Background(), next, &Config{
			SablierURL:      sablierURL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil))

		res := w.Result()
		defer res.Body.Close() //nolint:errcheck

		if res.StatusCode != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.StatusCode)
		}
	})
}

// TestSablierMiddleware_ServeHTTP_SSE tests Server-Sent Events streaming through the middleware.
//
// The critical behaviour under test is that SSE events are forwarded to the client when
// the session is ready, even though Traefik's internal proxy may call WriteHeader(200)
// without the httptrace WroteHeaders callback firing first (issue #29). The fix is that
// WriteHeader with any non-503 status code sets the responseWriter's ready flag so that
// subsequent Write calls are not silently discarded.
func TestSablierMiddleware_ServeHTTP_SSE(t *testing.T) {
	t.Run("streams SSE events when session is ready", func(t *testing.T) {
		// Sablier reports the session as ready.
		sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("ready"))
		}))
		defer sablierMockServer.Close()

		events := []string{
			"data: event1\n\n",
			"data: event2\n\n",
			"data: event3\n\n",
		}

		// The next handler simulates Traefik's SSE reverse-proxy behaviour:
		// it calls WriteHeader(200) then streams events via Write+Flush, without
		// triggering the httptrace WroteHeaders callback. Before the fix, every
		// Write was silently discarded because ready was never set to true.
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("responseWriter does not implement http.Flusher")
				return
			}
			for _, event := range events {
				_, _ = w.Write([]byte(event))
				flusher.Flush()
			}
		})

		sm, err := New(context.Background(), next, &Config{
			SablierURL:      sablierMockServer.URL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/sse/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		w := httptest.NewRecorder()

		sm.ServeHTTP(w, req)

		res := w.Result()
		defer res.Body.Close() //nolint:errcheck

		if res.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", res.StatusCode)
		}
		if got := res.Header.Get("Content-Type"); got != "text/event-stream" {
			t.Errorf("expected Content-Type text/event-stream, got %q", got)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if !strings.Contains(string(body), event) {
				t.Errorf("expected SSE event %q in response body, got:\n%s", event, body)
			}
		}
	})

	t.Run("shows Sablier waiting page for SSE request when session is not ready", func(t *testing.T) {
		sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "not-ready")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<h1>Starting up...</h1>"))
		}))
		defer sablierMockServer.Close()

		// This handler must not be reached when the session is not ready.
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: should-not-reach-client\n\n"))
		})

		sm, err := New(context.Background(), next, &Config{
			SablierURL:      sablierMockServer.URL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/sse/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		w := httptest.NewRecorder()

		sm.ServeHTTP(w, req)

		res := w.Result()
		defer res.Body.Close() //nolint:errcheck

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "should-not-reach-client") {
			t.Errorf("SSE events from backend must not reach the client when session is not ready, got:\n%s", body)
		}
		if !strings.Contains(string(body), "Starting up...") {
			t.Errorf("expected Sablier waiting page in response, got:\n%s", body)
		}
	})

	t.Run("shows Sablier waiting page when session is ready but backend returns 503", func(t *testing.T) {
		sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("container starting"))
		}))
		defer sablierMockServer.Close()

		// Backend is unavailable (no servers in the pool).
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		sm, err := New(context.Background(), next, &Config{
			SablierURL:      sablierMockServer.URL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/sse/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		w := httptest.NewRecorder()

		sm.ServeHTTP(w, req)

		res := w.Result()
		defer res.Body.Close() //nolint:errcheck

		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "container starting") {
			t.Errorf("expected Sablier waiting body when backend is 503, got:\n%s", body)
		}
	})

	// TestSablierMiddleware_ServeHTTP_SSE_RealServer verifies end-to-end SSE streaming
	// using a real HTTP server and client, closely matching how Traefik proxies SSE.
	t.Run("streams SSE events end-to-end via real HTTP server", func(t *testing.T) {
		sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("ready"))
		}))
		defer sablierMockServer.Close()

		events := []string{
			"data: hello\n\n",
			"data: world\n\n",
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("responseWriter does not implement http.Flusher")
				return
			}
			for _, event := range events {
				_, _ = w.Write([]byte(event))
				flusher.Flush()
			}
		})

		sm, err := New(context.Background(), next, &Config{
			SablierURL:      sablierMockServer.URL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		// Wrap the middleware in a real HTTP test server.
		srv := httptest.NewServer(sm)
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/sse/stream") //nolint:noctx
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close() //nolint:errcheck

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}
		if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
			t.Errorf("expected Content-Type text/event-stream, got %q", got)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			if !strings.Contains(string(body), event) {
				t.Errorf("expected SSE event %q in response body, got:\n%s", event, body)
			}
		}
	})
}

// TestSablierMiddleware_CacheControl verifies that every response generated by the
// plugin itself (waiting page, redirect) carries Cache-Control: no-store so that
// browsers and proxies cannot cache it. Responses from the actual backend are NOT
// modified and must not have the header injected by the plugin.
func TestSablierMiddleware_CacheControl(t *testing.T) {
	const noStore = "no-store"

	makeConfig := func(sablierURL string, dynamic bool) *Config {
		c := &Config{
			SablierURL:      sablierURL,
			Names:           "nginx",
			SessionDuration: "1m",
		}
		if dynamic {
			c.Dynamic = &DynamicConfiguration{}
		} else {
			c.Blocking = &BlockingConfiguration{}
		}
		return c
	}

	t.Run("waiting page response carries Cache-Control: no-store", func(t *testing.T) {
		sablierMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "not-ready")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<h1>Starting...</h1>"))
		}))
		defer sablierMock.Close()

		sm, err := New(context.Background(), http.NotFoundHandler(), makeConfig(sablierMock.URL, true), "middleware")
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/app", nil))

		if got := w.Result().Header.Get("Cache-Control"); got != noStore {
			t.Errorf("expected Cache-Control: %s, got %q", noStore, got)
		}
	})

	t.Run("redirect response carries Cache-Control: no-store", func(t *testing.T) {
		sablierMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("ready"))
		}))
		defer sablierMock.Close()

		// Blocking mode + 503 from backend → plugin issues a redirect.
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})

		sm, err := New(context.Background(), next, makeConfig(sablierMock.URL, false), "middleware")
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/app", nil))

		res := w.Result()
		if res.StatusCode != http.StatusFound {
			t.Fatalf("expected 302, got %d", res.StatusCode)
		}
		if got := res.Header.Get("Cache-Control"); got != noStore {
			t.Errorf("expected Cache-Control: %s, got %q", noStore, got)
		}
	})

	t.Run("backend response does not have Cache-Control injected by plugin", func(t *testing.T) {
		sablierMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("ready"))
		}))
		defer sablierMock.Close()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httptrace.ContextClientTrace(r.Context()).WroteHeaders()
			w.Header().Set("Content-Type", "application/json")
			// Backend deliberately does not set Cache-Control.
			_, _ = fmt.Fprint(w, `{"status":"ok"}`)
		})

		sm, err := New(context.Background(), next, makeConfig(sablierMock.URL, true), "middleware")
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/app", nil))

		if got := w.Result().Header.Get("Cache-Control"); got != "" {
			t.Errorf("plugin must not inject Cache-Control into backend responses, got %q", got)
		}
	})
}

func TestSablierMiddleware_IgnoreUserAgent(t *testing.T) {
	// notReadyMock returns a Sablier mock that always reports the session as not-ready.
	notReadyMock := func(t *testing.T) *httptest.Server {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Sablier-Session-Status", "not-ready")
			_, _ = w.Write([]byte("not ready"))
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	baseConfig := func(sablierURL string, patterns []string) *Config {
		return &Config{
			SablierURL:       sablierURL,
			Names:            "nginx",
			SessionDuration:  "1m",
			Dynamic:          &DynamicConfiguration{},
			IgnoreUserAgent: patterns,
		}
	}

	serve := func(t *testing.T, patterns []string, ua string) *http.Response {
		t.Helper()
		mock := notReadyMock(t)
		sm, err := New(context.Background(), http.NotFoundHandler(), baseConfig(mock.URL, patterns), "middleware")
		if err != nil {
			t.Fatalf("New() error: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if ua != "" {
			req.Header.Set("User-Agent", ua)
		}
		w := httptest.NewRecorder()
		sm.ServeHTTP(w, req)
		return w.Result()
	}

	// Single-value form: []string with one element.
	t.Run("single pattern matches substring", func(t *testing.T) {
		res := serve(t, []string{"curl"}, "curl/8.7.1")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != "request with user agent ignored as configured" {
			t.Errorf("unexpected body: %s", body)
		}
	})

	t.Run("single pattern does not match unrelated UA", func(t *testing.T) {
		res := serve(t, []string{"curl"}, "Mozilla/5.0")
		body, _ := io.ReadAll(res.Body)
		if string(body) == "request with user agent ignored as configured" {
			t.Error("Mozilla UA should not be ignored by curl pattern")
		}
	})

	t.Run("case-insensitive regexp matches UptimeRobot UA", func(t *testing.T) {
		res := serve(t, []string{"(?i)uptimerobot"}, "Mozilla/5.0+(compatible; UptimeRobot/2.0; http://www.uptimerobot.com/)")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}
		body, _ := io.ReadAll(res.Body)
		if string(body) != "request with user agent ignored as configured" {
			t.Errorf("unexpected body: %s", body)
		}
	})

	// Array form: []string with multiple elements.
	t.Run("multiple patterns in a list — first matches", func(t *testing.T) {
		res := serve(t, []string{"curl", "(?i)uptimerobot", "gitlab-runner"}, "curl/8.7.1")
		body, _ := io.ReadAll(res.Body)
		if string(body) != "request with user agent ignored as configured" {
			t.Errorf("curl should be ignored by first pattern, got: %s", body)
		}
	})

	t.Run("multiple patterns in a list — last matches", func(t *testing.T) {
		res := serve(t, []string{"curl", "(?i)uptimerobot", "gitlab-runner"}, "gitlab-runner/17.0")
		body, _ := io.ReadAll(res.Body)
		if string(body) != "request with user agent ignored as configured" {
			t.Errorf("gitlab-runner should be ignored by last pattern, got: %s", body)
		}
	})

	t.Run("multiple patterns in a list — none match", func(t *testing.T) {
		res := serve(t, []string{"curl", "(?i)uptimerobot", "gitlab-runner"}, "Mozilla/5.0")
		body, _ := io.ReadAll(res.Body)
		if string(body) == "request with user agent ignored as configured" {
			t.Error("Mozilla UA should not be ignored")
		}
	})

	t.Run("empty User-Agent is never ignored", func(t *testing.T) {
		res := serve(t, []string{".*"}, "") // .* matches everything, but empty UA must be exempt
		body, _ := io.ReadAll(res.Body)
		if string(body) == "request with user agent ignored as configured" {
			t.Error("empty User-Agent must not be treated as ignored")
		}
	})

	t.Run("invalid regexp returns error from New()", func(t *testing.T) {
		mock := notReadyMock(t)
		_, err := New(context.Background(), http.NotFoundHandler(), baseConfig(mock.URL, []string{"[invalid"}), "middleware")
		if err == nil {
			t.Fatal("expected error for invalid regexp, got nil")
		}
	})

	// Deserialization: StringOrStringSlice accepts both a JSON string (single
	// value, backward-compatible) and a JSON array (native list form).
	t.Run("JSON single string deserializes to one-element slice", func(t *testing.T) {
		var got StringOrStringSlice
		if err := json.Unmarshal([]byte(`"curl"`), &got); err != nil {
			t.Fatalf("UnmarshalJSON error: %v", err)
		}
		if len(got) != 1 || got[0] != "curl" {
			t.Errorf("expected [curl], got %v", got)
		}
	})

	t.Run("JSON array deserializes to multi-element slice", func(t *testing.T) {
		var got StringOrStringSlice
		if err := json.Unmarshal([]byte(`["curl","(?i)uptimerobot","gitlab-runner"]`), &got); err != nil {
			t.Fatalf("UnmarshalJSON error: %v", err)
		}
		want := StringOrStringSlice{"curl", "(?i)uptimerobot", "gitlab-runner"}
		if len(got) != len(want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("index %d: expected %q, got %q", i, want[i], got[i])
			}
		}
	})
}

func TestSablierMiddleware_KeepAlive(t *testing.T) {
	t.Run("sends keep-alive requests to Sablier while connection is held", func(t *testing.T) {
		// Each request to the mock Sablier server delivers a signal on this channel.
		sablierRequests := make(chan struct{}, 20)

		sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sablierRequests <- struct{}{}:
			default:
			}
			w.Header().Add("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("response from sablier"))
		}))
		defer sablierMockServer.Close()

		sm, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httptrace.ContextClientTrace(r.Context()).WroteHeaders()
			// Simulate a long-lived connection (SSE/WebSocket): block until disconnected.
			<-r.Context().Done()
		}), &Config{
			SablierURL:        sablierMockServer.URL,
			Names:             "nginx",
			SessionDuration:   "1m",
			Dynamic:           &DynamicConfiguration{},
			KeepAliveInterval: "20ms",
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		req := httptest.NewRequest(http.MethodGet, "/my-nginx", nil).WithContext(ctx)

		done := make(chan struct{})
		go func() {
			sm.ServeHTTP(httptest.NewRecorder(), req)
			close(done)
		}()

		// Consume the initial request to Sablier, then wait for two keep-alive pings.
		// The test will time out (and fail) if keep-alive never fires.
		<-sablierRequests
		<-sablierRequests
		<-sablierRequests

		// Cancel the context to disconnect the client and stop the goroutine.
		cancel()
		<-done
	})

	t.Run("does not send keep-alive requests when disabled", func(t *testing.T) {
		var count int32

		sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&count, 1)
			w.Header().Add("X-Sablier-Session-Status", "ready")
			_, _ = w.Write([]byte("response from sablier"))
		}))
		defer sablierMockServer.Close()

		sm, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			httptrace.ContextClientTrace(r.Context()).WroteHeaders()
			_, _ = fmt.Fprint(w, "response from service")
		}), &Config{
			SablierURL:      sablierMockServer.URL,
			Names:           "nginx",
			SessionDuration: "1m",
			Dynamic:         &DynamicConfiguration{},
			// KeepAliveInterval intentionally omitted
		}, "middleware")
		if err != nil {
			t.Fatal(err)
		}

		sm.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/my-nginx", nil))

		if got := atomic.LoadInt32(&count); got != 1 {
			t.Errorf("expected exactly 1 sablier request (no keep-alive), got %d", got)
		}
	})

	t.Run("returns error for invalid keepAliveInterval", func(t *testing.T) {
		_, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), &Config{
			SablierURL:        "http://sablier:10000",
			Names:             "nginx",
			Dynamic:           &DynamicConfiguration{},
			KeepAliveInterval: "invalid",
		}, "middleware")
		if err == nil {
			t.Error("expected error for invalid keepAliveInterval, got nil")
		}
	})

	t.Run("returns error for non-positive keepAliveInterval", func(t *testing.T) {
		_, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), &Config{
			SablierURL:        "http://sablier:10000",
			Names:             "nginx",
			Dynamic:           &DynamicConfiguration{},
			KeepAliveInterval: "-5s",
		}, "middleware")
		if err == nil {
			t.Error("expected error for non-positive keepAliveInterval, got nil")
		}
	})
}

func TestResponseWriter_WriteHeader_PreservesBackendSetCookie(t *testing.T) {
	recorder := httptest.NewRecorder()
	rw := newResponseWriter(recorder)

	// A handler running before the backend replied writes into the buffered
	// headers — this is what Traefik's sticky-session load balancer does
	// right before proxying the request.
	http.SetCookie(rw, &http.Cookie{Name: "lb", Value: "server-1", Path: "/"})

	// The reverse proxy sent the request (httptrace fired) and copies the
	// backend response headers onto the real writer.
	rw.ready = true
	rw.Header().Add("Set-Cookie", "state=abc123; Path=/; HttpOnly")

	rw.WriteHeader(http.StatusFound)

	cookies := recorder.Header().Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("expected both the sticky and the backend Set-Cookie headers, got %d: %v", len(cookies), cookies)
	}
}

func TestSablierMiddleware_ServeHTTP_PreservesBackendSetCookie(t *testing.T) {
	readySablier := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Sablier-Session-Status", "ready")
		_, _ = w.Write([]byte("ready"))
	}))
	defer readySablier.Close()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// What Traefik's sticky-session load balancer does before proxying:
		// the cookie lands in the buffered headers (writer not ready yet).
		http.SetCookie(w, &http.Cookie{Name: "lb", Value: "server-1", Path: "/"})

		// The proxy sends the request to the backend...
		httptrace.ContextClientTrace(r.Context()).WroteHeaders()

		// ...and copies the backend response headers and status.
		w.Header().Add("Set-Cookie", "state=abc123; Path=/; HttpOnly")
		w.WriteHeader(http.StatusFound)
	})

	sm, err := New(context.Background(), next, &Config{
		SablierURL:      readySablier.URL,
		Names:           "nginx",
		SessionDuration: "1m",
		Dynamic:         &DynamicConfiguration{},
	}, "middleware")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/my-nginx", nil))

	res := w.Result()
	defer res.Body.Close() //nolint:errcheck

	cookies := res.Header.Values("Set-Cookie")
	if len(cookies) != 2 {
		t.Fatalf("expected both the sticky and the backend Set-Cookie headers, got %d: %v", len(cookies), cookies)
	}
	for _, want := range []string{"lb=server-1", "state=abc123"} {
		found := false
		for _, c := range cookies {
			if strings.HasPrefix(c, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a Set-Cookie header starting with %q, got %v", want, cookies)
		}
	}
}
