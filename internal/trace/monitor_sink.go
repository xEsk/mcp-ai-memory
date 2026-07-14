package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// MonitorSink sends trace events to a local monitor endpoint.
type MonitorSink struct {
	client  *http.Client
	url     string
	timeout time.Duration
	debug   bool
}

// ValidateMonitorURL requires an HTTP endpoint on localhost or a literal loopback IP.
func ValidateMonitorURL(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse endpoint: %w", err)
	}
	if parsed.Scheme != "http" || parsed.Host == "" {
		return errors.New("trace endpoint must use http with a host")
	}
	if parsed.User != nil {
		return errors.New("trace endpoint must not include user information")
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("trace endpoint must use localhost or a literal loopback IP")
	}
	return nil
}

// NewMonitorSink creates a fail-silent HTTP sink for local monitor events.
func NewMonitorSink(endpoint string, timeout time.Duration, debug bool) (*MonitorSink, error) {
	if timeout <= 0 {
		timeout = 25 * time.Millisecond
	}
	dialer := &net.Dialer{Timeout: timeout}
	return newMonitorSink(endpoint, timeout, debug, dialer.DialContext)
}

// newMonitorSink builds a monitor sink with an injectable dialer for boundary tests.
func newMonitorSink(
	endpoint string,
	timeout time.Duration,
	debug bool,
	dialContext func(context.Context, string, string) (net.Conn, error),
) (*MonitorSink, error) {
	if err := ValidateMonitorURL(endpoint); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 25 * time.Millisecond
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			connection, err := dialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			if !isLoopbackRemoteAddress(connection.RemoteAddr()) {
				_ = connection.Close()
				return nil, fmt.Errorf("trace connection remote address %q is not loopback", connection.RemoteAddr())
			}
			return connection, nil
		},
	}
	return &MonitorSink{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		url:     endpoint,
		timeout: timeout,
		debug:   debug,
	}, nil
}

// isLoopbackRemoteAddress checks the effective peer of an established connection.
func isLoopbackRemoteAddress(address net.Addr) bool {
	if address == nil {
		return false
	}
	if tcpAddress, ok := address.(*net.TCPAddr); ok {
		return tcpAddress.IP.IsLoopback()
	}
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return false
	}
	return net.ParseIP(strings.Trim(host, "[]")).IsLoopback()
}

// Emit posts one event to the monitor and absorbs all errors.
func (s *MonitorSink) Emit(ctx context.Context, event Event) {
	if s == nil || s.url == "" {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		s.debugError("marshal trace event: %v", err)
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, s.url, bytes.NewReader(payload))
	if err != nil {
		s.debugError("create trace request: %v", err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		s.debugError("send trace event: %v", err)
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.debugError("send trace event: status %s", response.Status)
	}
}

func (s *MonitorSink) debugError(format string, args ...any) {
	if !s.debug {
		return
	}
	fmt.Fprintf(os.Stderr, "trace: "+format+"\n", args...)
}
