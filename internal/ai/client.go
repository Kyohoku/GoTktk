package ai

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gotik/internal/config"
)

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(cfg *config.AIConfig) (*HTTPClient, error) {
	if cfg == nil {
		return nil, errors.New("ai config is nil")
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("ai host is required")
	}
	if cfg.Port <= 0 {
		return nil, errors.New("ai port must be greater than 0")
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 30
	}

	baseURL := fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port)

	return &HTTPClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
		},
	}, nil
}

func (c *HTTPClient) BaseURL() string {
	return c.baseURL
}

func (c *HTTPClient) buildURL(path string) string {
	if strings.HasPrefix(path, "/") {
		return c.baseURL + path
	}
	return c.baseURL + "/" + path
}
