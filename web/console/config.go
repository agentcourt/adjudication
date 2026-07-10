package console

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	ListenAddr      string
	WebBearerToken  string
	RequestTimeout  time.Duration
	Systems         map[string]SystemConfig
	MaxServiceBytes int64
}

type SystemConfig struct {
	ID          string
	Label       string
	BaseURL     string
	BearerToken string
	Scopes      []ScopeConfig
}

type ScopeConfig struct {
	ID           string
	Label        string
	BasePath     string
	ManageAction string
	Description  string
}

func DefaultConfig() Config {
	return Config{
		ListenAddr:      "127.0.0.1:19990",
		RequestTimeout:  30 * time.Second,
		MaxServiceBytes: 32 << 20,
		Systems: map[string]SystemConfig{
			"adc": {
				ID:      "adc",
				Label:   "ADC",
				BaseURL: "http://127.0.0.1:19870",
				Scopes: []ScopeConfig{
					{ID: "clerk", Label: "Clerk", BasePath: "/clerk/v1/cases", ManageAction: "kill", Description: "Full ADC runs and direct-mode ADC cases."},
					{ID: "api", Label: "API Alias", BasePath: "/api/v1/cases", ManageAction: "kill", Description: "ADC service alias for the same case-management API."},
				},
			},
			"arb": {
				ID:      "arb",
				Label:   "ARB",
				BaseURL: "http://127.0.0.1:19770",
				Scopes: []ScopeConfig{
					{ID: "clerk", Label: "Clerk", BasePath: "/clerk/v1/cases", ManageAction: "kill", Description: "Full AAR runs."},
					{ID: "direct", Label: "Direct", BasePath: "/api/v1/cases", ManageAction: "cancel", Description: "Direct AAR case processes."},
				},
			},
			"arbd": {
				ID:      "arbd",
				Label:   "AARD",
				BaseURL: "http://127.0.0.1:19790",
				Scopes: []ScopeConfig{
					{ID: "clerk", Label: "Clerk", BasePath: "/clerk/v1/cases", ManageAction: "kill", Description: "Full AARD runs."},
					{ID: "direct", Label: "Direct", BasePath: "/api/v1/cases", ManageAction: "cancel", Description: "Direct AARD case processes."},
				},
			},
		},
	}
}

func (c Config) normalized() (Config, error) {
	if c.ListenAddr == "" {
		c.ListenAddr = "127.0.0.1:19990"
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 30 * time.Second
	}
	if c.MaxServiceBytes <= 0 {
		c.MaxServiceBytes = 32 << 20
	}
	for id, sys := range c.Systems {
		sys.ID = strings.TrimSpace(sys.ID)
		if sys.ID == "" {
			sys.ID = id
		}
		sys.Label = strings.TrimSpace(sys.Label)
		if sys.Label == "" {
			sys.Label = strings.ToUpper(sys.ID)
		}
		sys.BaseURL = strings.TrimRight(strings.TrimSpace(sys.BaseURL), "/")
		for i := range sys.Scopes {
			scope := &sys.Scopes[i]
			scope.ID = strings.TrimSpace(scope.ID)
			scope.Label = strings.TrimSpace(scope.Label)
			scope.BasePath = "/" + strings.Trim(strings.TrimSpace(scope.BasePath), "/")
			scope.ManageAction = strings.TrimSpace(scope.ManageAction)
			if scope.ID == "" || scope.BasePath == "/" || scope.ManageAction == "" {
				return Config{}, fmt.Errorf("system %s has incomplete scope configuration", sys.ID)
			}
		}
		c.Systems[id] = sys
	}
	return c, nil
}

func (c Config) system(id string) (SystemConfig, bool) {
	sys, ok := c.Systems[id]
	return sys, ok
}

func (s SystemConfig) scope(id string) (ScopeConfig, bool) {
	for _, scope := range s.Scopes {
		if scope.ID == id {
			return scope, true
		}
	}
	return ScopeConfig{}, false
}

func (s SystemConfig) configured() bool {
	return strings.TrimSpace(s.BaseURL) != ""
}
