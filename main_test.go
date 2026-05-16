package sablier_traefik_plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
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
	}
	tests := []struct {
		name     string
		fields   fields
		sablier  sablier
		expected string
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
					SessionDuration: "1m",
					Dynamic:         &DynamicConfiguration{},
					IgnoreUserAgent: "curl",
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
					SessionDuration: "1m",
					Dynamic:         &DynamicConfiguration{},
					IgnoreUserAgent: "curl",
				},
				Headers: &map[string]string{
					"User-Agent": "Mozilla",
				},
			},
			expected: "response from service",
			code:     200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sablierMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.sablier.headers {
					w.Header().Add(key, value)
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

			req := httptest.NewRequest(http.MethodGet, "/my-nginx", nil)
			w := httptest.NewRecorder()

			if tt.fields.Headers != nil {
				for k, v := range *tt.fields.Headers {
					req.Header.Add(k, v)
				}
			}

			sm.ServeHTTP(w, req)

			res := w.Result()
			defer func() {
				_ = res.Body.Close()
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
		defer res.Body.Close()

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
		defer res.Body.Close()

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
		defer res.Body.Close()

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
		defer resp.Body.Close()

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
