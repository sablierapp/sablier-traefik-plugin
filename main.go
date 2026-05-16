package sablier_traefik_plugin

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"regexp"
	"strings"
	"time"
)

type SablierMiddleware struct {
	client            *http.Client
	request           *http.Request
	next              http.Handler
	useRedirect       bool
	failOpen          bool
	ignoreUserAgents  []*regexp.Regexp
	keepAliveInterval time.Duration
}

// New function creates the configuration
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	req, err := config.BuildRequest(name)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "sablier-traefik-plugin/"+Version)

	var keepAliveInterval time.Duration
	if config.KeepAliveInterval != "" {
		keepAliveInterval, err = time.ParseDuration(config.KeepAliveInterval)
		if err != nil {
			return nil, fmt.Errorf("error parsing keepAliveInterval: %v", err)
		}
		if keepAliveInterval <= 0 {
			return nil, fmt.Errorf("keepAliveInterval must be positive")
		}
	}

	ignoreUserAgents, err := compileIgnoreUserAgents(config.IgnoreUserAgent)
	if err != nil {
		return nil, err
	}

	return &SablierMiddleware{
		request: req,
		client:  &http.Client{},
		next:    next,
		// there is no way to make blocking work in traefik without redirect so let's make it default
		useRedirect:       config.Blocking != nil,
		failOpen:          config.FailOpen,
		ignoreUserAgents:  ignoreUserAgents,
		keepAliveInterval: keepAliveInterval,
	}, nil
}

// compileIgnoreUserAgents compiles each pattern in the ignoreUserAgent config
// list into a regexp. Each element of patterns is treated as one Go regexp.
func compileIgnoreUserAgents(patterns []string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid ignoreUserAgent pattern %q: %v", pattern, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

func (sm *SablierMiddleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if sm.matchesIgnoredUserAgent(req.Header.Get("User-Agent")) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("request with user agent ignored as configured"))
		return
	}

	sablierRequest := sm.request.Clone(req.Context())

	resp, err := sm.client.Do(sablierRequest)
	if err != nil {
		if sm.failOpen {
			sm.next.ServeHTTP(rw, req)
			return
		}
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	conditonalResponseWriter := newResponseWriter(rw)

	useRedirect := false

	if resp.Header.Get("X-Sablier-Session-Status") == "ready" {
		// Check if the backend already received request data
		if sm.keepAliveInterval > 0 {
			go sm.keepAlive(req.Context())
		}
		trace := &httptrace.ClientTrace{
			WroteHeaders: func() {
				conditonalResponseWriter.ready = true
			},
			WroteRequest: func(info httptrace.WroteRequestInfo) {
				conditonalResponseWriter.ready = true
			},
		}
		newCtx := httptrace.WithClientTrace(req.Context(), trace)
		sm.next.ServeHTTP(conditonalResponseWriter, req.WithContext(newCtx))
		useRedirect = sm.useRedirect
	}

	if !conditonalResponseWriter.ready {
		conditonalResponseWriter.ready = true
		// Prevent browsers and proxies from caching the waiting page or redirect.
		// Without this, a cached 200 or 3xx response would keep being served even
		// after the container is ready, causing the stuck-page bug (issue #28) and
		// the stale-redirect bug (issue #30).
		conditonalResponseWriter.Header().Set("Cache-Control", "no-store")
		if useRedirect {
			conditonalResponseWriter.Header().Set("Location", req.URL.String())

			status := http.StatusFound
			if req.Method != http.MethodGet {
				status = http.StatusTemporaryRedirect
			}

			conditonalResponseWriter.WriteHeader(status)
			_, err := conditonalResponseWriter.Write([]byte(http.StatusText(status)))
			if err != nil {
				http.Error(conditonalResponseWriter, err.Error(), http.StatusInternalServerError)
			}
		} else {
			conditonalResponseWriter.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			conditonalResponseWriter.WriteHeader(resp.StatusCode)
			_, err := io.Copy(conditonalResponseWriter, resp.Body)
			if err != nil {
				http.Error(conditonalResponseWriter, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

func newResponseWriter(rw http.ResponseWriter) *responseWriter {
	return &responseWriter{
		responseWriter: rw,
		headers:        make(http.Header),
	}
}

type responseWriter struct {
	responseWriter http.ResponseWriter
	headers        http.Header
	ready          bool
}

func (r *responseWriter) Header() http.Header {
	if r.ready {
		return r.responseWriter.Header()
	}
	return r.headers
}

func (r *responseWriter) Write(buf []byte) (int, error) {
	if !r.ready {
		return len(buf), nil
	}
	return r.responseWriter.Write(buf)
}

func (r *responseWriter) WriteHeader(code int) {
	if !r.ready && code == http.StatusServiceUnavailable {
		// We get a 503 HTTP Status Code when there is no backend server in the pool
		// to which the request could be sent.  Also, note that r.ready
		// will never return false in case there was a connection established to
		// the backend server and so we can be sure that the 503 was produced
		// inside Traefik already
		return
	}

	// Once we commit to writing any non-503 status, all subsequent Write calls
	// must reach the client. This is critical for streaming protocols (SSE,
	// WebSocket handshake) where Traefik may call WriteHeader(200) and then
	// stream the body without the httptrace WroteHeaders callback firing.
	r.ready = true

	headers := r.responseWriter.Header()
	for header, value := range r.headers {
		headers[header] = value
	}

	r.responseWriter.WriteHeader(code)
}

func (r *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.responseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("%T is not a http.Hijacker", r.responseWriter)
	}
	return hijacker.Hijack()
}

func (r *responseWriter) Flush() {
	if flusher, ok := r.responseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// matchesIgnoredUserAgent reports whether ua matches any of the compiled
// ignoreUserAgent patterns. An empty User-Agent is never considered a match.
func (sm *SablierMiddleware) matchesIgnoredUserAgent(ua string) bool {
	if ua == "" {
		return false
	}
	for _, re := range sm.ignoreUserAgents {
		if re.MatchString(ua) {
			return true
		}
	}
	return false
}

// keepAlive periodically sends requests to Sablier to renew the session while a
// long-lived connection (SSE, WebSocket, long-polling) is held open. It stops
// as soon as ctx is done, i.e. when the client disconnects.
func (sm *SablierMiddleware) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(sm.keepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			keepAliveReq := sm.request.Clone(context.Background())
			resp, err := sm.client.Do(keepAliveReq)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	}
}
