package sablier_traefik_plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
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
					Names:           "nginx",
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
					Names:           "nginx",
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
