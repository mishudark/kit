package serverutil

import (
	"net/http"
	"time"
)

// Option configures an HTTP Server.
type Option func(*http.Server)

// NewServer creates an HTTP Server with pre-configured timeouts
func NewServer(addr string, h http.Handler, opts ...Option) *http.Server {
	srv := http.Server{
		Addr:              addr,
		Handler:           h,
		ReadTimeout:       25 * time.Second,
		WriteTimeout:      40 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       5 * time.Minute,
		MaxHeaderBytes:    1 << 18, // 0.25 MB (262144 bytes)
	}

	for _, opt := range opts {
		opt(&srv)
	}

	return &srv
}
