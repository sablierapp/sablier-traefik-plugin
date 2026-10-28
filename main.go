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
	"sync"
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

	transportOpts, err := parseTransportOptions(config)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	if transportOpts != (transportOptions{}) {
		client.Transport = sharedTransport(transportOpts)
	}

	return &SablierMiddleware{
		request: req,
		client:  client,
		next:    next,
		// there is no way to make blocking work in traefik without redirect so let's make it default
		useRedirect:       config.Blocking != nil,
		failOpen:          config.FailOpen,
		ignoreUserAgents:  ignoreUserAgents,
		keepAliveInterval: keepAliveInterval,
	}, nil
}

// transportOptions holds the settings that need a custom HTTP transport to Sablier.
// The zero value means Go's default transport is used.
type transportOptions struct {
	dialTimeout time.Duration
}

// parseTransportOptions reads the transport settings from config.
func parseTransportOptions(config *Config) (transportOptions, error) {
	var opts transportOptions
	if config.DialTimeout != "" {
		dialTimeout, err := time.ParseDuration(config.DialTimeout)
		if err != nil {
			return opts, fmt.Errorf("error parsing dialTimeout: %v", err)
		}
		if dialTimeout <= 0 {
			return opts, fmt.Errorf("dialTimeout must be positive")
		}
		opts.dialTimeout = dialTimeout
	}
	return opts, nil
}

var (
	sharedTransportsMu sync.Mutex
	sharedTransports   = map[transportOptions]*http.Transport{}
)

// sharedTransport returns a copy of http.DefaultTransport with opts applied.
//
// Transports are shared per set of options because Traefik calls New for every middleware
// on each configuration reload, and a transport per call would leave its idle
// connections open until they time out.
func sharedTransport(opts transportOptions) *http.Transport {
	sharedTransportsMu.Lock()
	defer sharedTransportsMu.Unlock()

	if transport, ok := sharedTransports[opts]; ok {
		return transport
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()

	// dialTimeout bounds only connection setup. Without it a connection to an unreachable
	// Sablier address waits for the default 30s dial timeout. Requests can still take long,
	// as they do with the blocking strategy.
	if opts.dialTimeout > 0 {
		dialer := &net.Dialer{Timeout: opts.dialTimeout, KeepAlive: 30 * time.Second}
		// A closure rather than the method value dialer.DialContext: Yaegi, which Traefik uses
		// to run plugins, gives that method value the wrong type and the plugin fails to load.
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		}
	}

	sharedTransports[opts] = transport
	return transport
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

	defer closeBody(resp.Body)

	conditonalResponseWriter := newResponseWriter(rw)

	useRedirect := false

	if resp.Header.Get("X-Sablier-Session-Status") == "ready" {
		// Check if the backend already received request data
		if sm.keepAliveInterval > 0 {
			go sm.keepAlive(req.Context())
		}
		trace := &httptrace.ClientTrace{
			GetConn: func(hostPort string) {
				conditonalResponseWriter.connecting = true
			},
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
	// connecting is set once the proxy tries to connect to a backend server.
	connecting bool
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
	if !r.ready && r.noBackendYet(code) {
		// r.ready is still false, so the request never reached a backend server
		// and this error was produced inside Traefik because there is no backend
		// server to forward the request to yet (see noBackendYet).
		//
		// The whole error response is discarded: its body is dropped by Write
		// and its status is never forwarded, so the headers it wrote must be
		// dropped too. http.Error — used by Traefik's load balancer when the
		// service has no available server — writes "Content-Type: text/plain;
		// charset=utf-8" and "X-Content-Type-Options: nosniff" into the
		// buffered headers; left in place, they would leak into the waiting
		// page on the final flush and override its text/html Content-Type
		// (the page then renders as plain text).
		r.headers = make(http.Header)
		return
	}

	// Once we commit to writing any other status, all subsequent Write calls
	// must reach the client. This is critical for streaming protocols (SSE,
	// WebSocket handshake) where Traefik may call WriteHeader(200) and then
	// stream the body without the httptrace WroteHeaders callback firing.
	r.ready = true

	headers := r.responseWriter.Header()
	for header, value := range r.headers {
		// Set-Cookie is multi-valued: each value is an independent cookie.
		// The buffered map may hold cookies written before the backend
		// replied (e.g. Traefik's sticky-session cookie, written by the
		// load balancer before proxying), while the real writer already
		// holds the cookies of the backend response. Overwriting would
		// drop the backend cookies (e.g. an OIDC state cookie), so append
		// instead.
		if header == "Set-Cookie" {
			headers[header] = append(headers[header], value...)
		} else {
			headers[header] = value
		}
	}

	r.responseWriter.WriteHeader(code)
}

// noBackendYet reports whether code is an error Traefik answers with when there
// is no backend server to forward the request to yet:
//   - 503 from the load balancer when there is no server in the pool.
//   - 500 from the proxy when it fails before trying to connect to the server,
//     which happens when the server has no URL because the container was not
//     running when the request arrived (issue #43).
//
// Errors raised once the proxy tries to connect to a server, such as a 502 when
// nothing listens on the configured port, are forwarded: retrying cannot fix
// them, and the plugin would keep redirecting to itself or showing the waiting
// page.
func (r *responseWriter) noBackendYet(code int) bool {
	switch code {
	case http.StatusServiceUnavailable:
		return true
	case http.StatusInternalServerError:
		return !r.connecting
	}
	return false
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

// closeBody reads a response from Sablier to the end before closing it, so its
// connection goes back to the pool for the next request. Go 1.27 does this on
// Close, but Traefik releases built with older Go don't:
// https://go.dev/blog/http-connection-reuse
func closeBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
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
				closeBody(resp.Body)
			}
		}
	}
}
