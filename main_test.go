package sablier_traefik_plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
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
