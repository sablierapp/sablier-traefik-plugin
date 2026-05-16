package sablier_traefik_plugin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// StringOrStringSlice is a []string config field that can be deserialized from
// either a single JSON string or a JSON/YAML array of strings. Traefik's
// configuration system also coerces a single string into a one-element slice
// when loading YAML or Docker-label configs (weak-decode behaviour), so both
// forms below are accepted:
//
//	ignoreUserAgent: curl                # single value (backward-compatible)
//	ignoreUserAgent:                     # list
//	  - curl
//	  - "(?i)uptimerobot"
type StringOrStringSlice []string

// UnmarshalJSON implements json.Unmarshaler so that this field accepts both a
// JSON string ("curl") and a JSON array (["curl","(?i)uptimerobot"]).
func (s *StringOrStringSlice) UnmarshalJSON(data []byte) error {
	// Try JSON array first.
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}
	// Fall back to a single JSON string.
	var single string
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("ignoreUserAgent: expected a string or an array of strings")
	}
	*s = StringOrStringSlice{single}
	return nil
}

type DynamicConfiguration struct {
	DisplayName      string `yaml:"displayname"`
	ShowDetails      *bool  `yaml:"showDetails"`
	Theme            string `yaml:"theme"`
	RefreshFrequency string `yaml:"refreshFrequency"`
}

type BlockingConfiguration struct {
	Timeout string `yaml:"timeout"`
}

type Config struct {
	SablierURL string `yaml:"sablierUrl"`
	// Deprecated: use Group instead
	Names             string `yaml:"names"`
	Group             string `yaml:"group"`
	SessionDuration   string `yaml:"sessionDuration"`
	KeepAliveInterval string `yaml:"keepAliveInterval"`
	FailOpen          bool   `yaml:"failOpen"`
	splittedNames     []string
	Dynamic           *DynamicConfiguration  `yaml:"dynamic"`
	Blocking          *BlockingConfiguration `yaml:"blocking"`
	IgnoreUserAgent   StringOrStringSlice    `yaml:"ignoreUserAgent" json:"ignoreUserAgent"`
}

func CreateConfig() *Config {
	return &Config{
		SablierURL:        "http://sablier:10000",
		Names:             "",
		Group:             "",
		SessionDuration:   "",
		KeepAliveInterval: "",
		FailOpen:          false,
		splittedNames:     []string{},
		Dynamic:           nil,
		Blocking:          nil,
		IgnoreUserAgent:   StringOrStringSlice{},
	}
}

func (c *Config) BuildRequest(middlewareName string) (*http.Request, error) {

	if len(c.SablierURL) == 0 {
		return nil, fmt.Errorf("sablierURL cannot be empty")
	}

	names := strings.Split(c.Names, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	if len(names) >= 1 && len(names[0]) > 0 {
		c.splittedNames = names
	}

	if len(c.splittedNames) == 0 && len(c.Group) == 0 {
		return nil, fmt.Errorf("you must specify at least one name or a group")
	}

	if c.Dynamic != nil && c.Blocking != nil {
		return nil, fmt.Errorf("only supply one strategy: dynamic or blocking")
	}

	if c.Dynamic != nil {
		return c.buildDynamicRequest(middlewareName)
	} else if c.Blocking != nil {
		return c.buildBlockingRequest()
	}
	return nil, fmt.Errorf("no strategy configured")
}

func (c *Config) buildDynamicRequest(middlewareName string) (*http.Request, error) {
	if c.Dynamic == nil {
		return nil, fmt.Errorf("dynamic config is nil")
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/strategies/dynamic", c.SablierURL), nil)
	if err != nil {
		return nil, err
	}

	q := request.URL.Query()

	if c.SessionDuration != "" {
		_, err = time.ParseDuration(c.SessionDuration)

		if err != nil {
			return nil, fmt.Errorf("error parsing dynamic.sessionDuration: %v", err)
		}

		q.Add("session_duration", c.SessionDuration)
	}

	for _, name := range c.splittedNames {
		q.Add("names", name)
	}

	if c.Group != "" {
		q.Add("group", c.Group)
	}

	if c.Dynamic.DisplayName != "" {
		q.Add("display_name", c.Dynamic.DisplayName)
	} else {
		// display name defaults as middleware name
		q.Add("display_name", middlewareName)
	}

	if c.Dynamic.Theme != "" {
		q.Add("theme", c.Dynamic.Theme)
	}

	if c.Dynamic.RefreshFrequency != "" {
		_, err := time.ParseDuration(c.Dynamic.RefreshFrequency)

		if err != nil {
			return nil, fmt.Errorf("error parsing dynamic.refreshFrequency: %v", err)
		}

		q.Add("refresh_frequency", c.Dynamic.RefreshFrequency)
	}

	if c.Dynamic.ShowDetails != nil {
		q.Add("show_details", strconv.FormatBool(*c.Dynamic.ShowDetails))
	}

	request.URL.RawQuery = q.Encode()

	return request, nil
}

func (c *Config) buildBlockingRequest() (*http.Request, error) {
	if c.Blocking == nil {
		return nil, fmt.Errorf("blocking config is nil")
	}

	request, err := http.NewRequest("GET", fmt.Sprintf("%s/api/strategies/blocking", c.SablierURL), nil)
	if err != nil {
		return nil, err
	}

	q := request.URL.Query()

	if c.SessionDuration != "" {
		_, err = time.ParseDuration(c.SessionDuration)

		if err != nil {
			return nil, fmt.Errorf("error parsing blocking.sessionDuration: %v", err)
		}

		q.Add("session_duration", c.SessionDuration)
	}

	for _, name := range c.splittedNames {
		q.Add("names", name)
	}

	if c.Group != "" {
		q.Add("group", c.Group)
	}

	if c.Blocking.Timeout != "" {
		_, err := time.ParseDuration(c.Blocking.Timeout)

		if err != nil {
			return nil, fmt.Errorf("error parsing blocking.timeout: %v", err)
		}

		q.Add("timeout", c.Blocking.Timeout)
	}

	request.URL.RawQuery = q.Encode()

	return request, nil
}
