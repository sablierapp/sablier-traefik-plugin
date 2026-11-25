package sablier_traefik_plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync"
	"testing"
	"time"
)

// autoSablier is a Sablier stand-in that records which strategy endpoint each
// request reached and answers with the given session status.
type autoSablier struct {
	mu     sync.Mutex
	paths  []string
	server *httptest.Server
}

func newAutoSablier(t *testing.T, status string) *autoSablier {
	t.Helper()
	s := &autoSablier{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.Path+"?"+r.URL.RawQuery)
		s.mu.Unlock()
		w.Header().Set("X-Sablier-Session-Status", status)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("waiting page"))
	}))
	// A closure rather than the method value s.server.Close, which Yaegi mistypes.
	t.Cleanup(func() { s.server.Close() })
	return s
}

func (s *autoSablier) lastPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.paths) == 0 {
		return ""
	}
	return s.paths[len(s.paths)-1]
}

// autoConfig configures both strategies, which is what turns the auto mode on.
func autoConfig(sablierURL string) *Config {
	return &Config{
		SablierURL:      sablierURL,
		Group:           "grafana",
		SessionDuration: "15m",
		Dynamic:         &DynamicConfiguration{DisplayName: "Grafana"},
		Blocking:        &BlockingConfiguration{Timeout: "2m"},
	}
}

var reachedBackend = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	httptrace.ContextClientTrace(r.Context()).WroteHeaders()
	_, _ = w.Write([]byte("response from service"))
})

func TestAutoStrategy_PicksStrategyPerRequest(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		headers  map[string]string
		strategy string
	}{
		{
			name:     "browser navigation gets the waiting page",
			headers:  map[string]string{"Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "document", "Accept": "text/html,application/xhtml+xml"},
			strategy: "dynamic",
		},
		{
			name:     "navigation into an iframe gets the waiting page",
			headers:  map[string]string{"Sec-Fetch-Mode": "navigate", "Sec-Fetch-Dest": "iframe"},
			strategy: "dynamic",
		},
		{
			name:     "fetch from an open page is held",
			headers:  map[string]string{"Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty", "Accept": "application/json"},
			strategy: "blocking",
		},
		{
			name:     "script or image subresource is held",
			headers:  map[string]string{"Sec-Fetch-Mode": "no-cors", "Sec-Fetch-Dest": "script", "Accept": "*/*"},
			strategy: "blocking",
		},
		{
			name:     "websocket upgrade is held",
			headers:  map[string]string{"Sec-Fetch-Mode": "websocket", "Upgrade": "websocket", "Connection": "Upgrade"},
			strategy: "blocking",
		},
		{
			name:     "html GET without fetch metadata gets the waiting page",
			headers:  map[string]string{"Accept": "text/html,application/xhtml+xml,*/*;q=0.8"},
			strategy: "dynamic",
		},
		{
			name:     "json GET without fetch metadata is held",
			headers:  map[string]string{"Accept": "application/json"},
			strategy: "blocking",
		},
		{
			name:     "curl with no Accept preference is held",
			headers:  map[string]string{"Accept": "*/*", "User-Agent": "curl/8.11.1"},
			strategy: "blocking",
		},
		{
			name:     "html POST without fetch metadata is held",
			method:   http.MethodPost,
			headers:  map[string]string{"Accept": "text/html"},
			strategy: "blocking",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sablier := newAutoSablier(t, "ready")
			sm, err := New(context.Background(), reachedBackend, autoConfig(sablier.server.URL), "grafana-auto")
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			method := http.MethodGet
			if tt.method != "" {
				method = tt.method
			}
			req := httptest.NewRequest(method, "/api/ds/query", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			sm.ServeHTTP(httptest.NewRecorder(), req)

			got := sablier.lastPath()
			if !strings.HasPrefix(got, "/api/strategies/"+tt.strategy+"?") {
				t.Errorf("Sablier was asked %q, want the %s strategy", got, tt.strategy)
			}
		})
	}
}

func TestAutoStrategy_KeepsEachStrategyParameters(t *testing.T) {
	sablier := newAutoSablier(t, "ready")
	sm, err := New(context.Background(), reachedBackend, autoConfig(sablier.server.URL), "grafana-auto")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	nav := httptest.NewRequest(http.MethodGet, "/", nil)
	nav.Header.Set("Sec-Fetch-Mode", "navigate")
	sm.ServeHTTP(httptest.NewRecorder(), nav)
	if got := sablier.lastPath(); !strings.Contains(got, "display_name=Grafana") || !strings.Contains(got, "group=grafana") {
		t.Errorf("dynamic request = %q, want display_name=Grafana and group=grafana", got)
	}

	xhr := httptest.NewRequest(http.MethodGet, "/api/ds/query", nil)
	xhr.Header.Set("Sec-Fetch-Mode", "cors")
	sm.ServeHTTP(httptest.NewRecorder(), xhr)
	if got := sablier.lastPath(); !strings.Contains(got, "timeout=2m") || !strings.Contains(got, "group=grafana") || !strings.Contains(got, "session_duration=15m") {
		t.Errorf("blocking request = %q, want timeout=2m, group=grafana and session_duration=15m", got)
	}
}

// A request that Sablier lets through but that never reaches the backend (the
// container is up but its load balancer entry is not yet there) is retried by
// redirect under the blocking strategy and answered with the waiting page under
// the dynamic one. In auto mode, each request follows its own strategy.
func TestAutoStrategy_BackendNotReachedFollowsChosenStrategy(t *testing.T) {
	notReached := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	sablier := newAutoSablier(t, "ready")
	sm, err := New(context.Background(), notReached, autoConfig(sablier.server.URL), "grafana-auto")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	nav := httptest.NewRequest(http.MethodGet, "/d/abc", nil)
	nav.Header.Set("Sec-Fetch-Mode", "navigate")
	navRec := httptest.NewRecorder()
	sm.ServeHTTP(navRec, nav)
	if navRec.Code != http.StatusOK || navRec.Body.String() != "waiting page" {
		t.Errorf("navigation got %d %q, want 200 with the waiting page", navRec.Code, navRec.Body.String())
	}

	xhr := httptest.NewRequest(http.MethodGet, "/api/ds/query", nil)
	xhr.Header.Set("Sec-Fetch-Mode", "cors")
	xhrRec := httptest.NewRecorder()
	sm.ServeHTTP(xhrRec, xhr)
	if xhrRec.Code != http.StatusFound || xhrRec.Header().Get("Location") != "/api/ds/query" {
		t.Errorf("fetch got %d Location=%q, want 302 to /api/ds/query", xhrRec.Code, xhrRec.Header().Get("Location"))
	}
}

func TestAutoStrategy_SingleStrategyIsUnchanged(t *testing.T) {
	for _, strategy := range []string{"dynamic", "blocking"} {
		t.Run(strategy, func(t *testing.T) {
			sablier := newAutoSablier(t, "ready")
			config := &Config{SablierURL: sablier.server.URL, Group: "grafana"}
			if strategy == "dynamic" {
				config.Dynamic = &DynamicConfiguration{}
			} else {
				config.Blocking = &BlockingConfiguration{}
			}
			sm, err := New(context.Background(), reachedBackend, config, "single")
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			// A navigation and a fetch both go to the one configured strategy.
			for _, mode := range []string{"navigate", "cors"} {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("Sec-Fetch-Mode", mode)
				sm.ServeHTTP(httptest.NewRecorder(), req)
				if got := sablier.lastPath(); !strings.HasPrefix(got, "/api/strategies/"+strategy+"?") {
					t.Errorf("Sec-Fetch-Mode %s: Sablier was asked %q, want %s", mode, got, strategy)
				}
			}
		})
	}
}

// A long-lived connection renews its session with the strategy that let it
// through: a WebSocket held by the blocking strategy must not renew through the
// dynamic one, which answers with an HTML page.
func TestAutoStrategy_KeepAliveRenewsWithChosenStrategy(t *testing.T) {
	sablier := newAutoSablier(t, "ready")
	config := autoConfig(sablier.server.URL)
	config.KeepAliveInterval = "10ms"

	held := make(chan struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httptrace.ContextClientTrace(r.Context()).WroteHeaders()
		<-held
	})
	sm, err := New(context.Background(), next, config, "grafana-auto")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/live/ws", nil).WithContext(ctx)
	req.Header.Set("Sec-Fetch-Mode", "websocket")
	done := make(chan struct{})
	go func() {
		sm.ServeHTTP(httptest.NewRecorder(), req)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		sablier.mu.Lock()
		n := len(sablier.paths)
		sablier.mu.Unlock()
		if n >= 3 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("got %d requests to Sablier, want the first one plus keep-alives", n)
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	close(held)
	<-done

	sablier.mu.Lock()
	defer sablier.mu.Unlock()
	for i, p := range sablier.paths {
		if !strings.HasPrefix(p, "/api/strategies/blocking?") {
			t.Errorf("request %d to Sablier was %q, want the blocking strategy", i, p)
		}
	}
}

// TestSablierMiddleware_NoStrategyDefaultsToDynamic covers issue #11: a
// middleware that sets neither dynamic nor blocking used to fail to load, so a
// configuration made of labels had to set an option such as
// dynamic.displayName just to pick a strategy.
func TestSablierMiddleware_NoStrategyDefaultsToDynamic(t *testing.T) {
	sablier := newAutoSablier(t, "not-ready")

	// What Traefik builds from labels that set no strategy option.
	config := CreateConfig()
	config.SablierURL = sablier.server.URL
	config.Group = "whoami"
	config.SessionDuration = "1m"

	sm, err := New(context.Background(), reachedBackend, config, "whoami-sablier@docker")
	if err != nil {
		t.Fatalf("expected the middleware to load without a strategy, got %v", err)
	}

	w := httptest.NewRecorder()
	sm.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := sablier.lastPath(); !strings.HasPrefix(got, "/api/strategies/dynamic?") || !strings.Contains(got, "display_name=whoami-sablier%40docker") {
		t.Errorf("expected the dynamic strategy with the middleware name as display name, got %q", got)
	}
	if w.Code != http.StatusOK || w.Body.String() != "waiting page" {
		t.Errorf("expected the waiting page, got %d %q", w.Code, w.Body.String())
	}
}
