package sourcefetch

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lowcode-faas/internal/config"
)

func FetchSourceFromURL(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("source_url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse source_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("source_url scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("source_url host is required")
	}
	if err := validateTarget(ctx, u); err != nil {
		return "", err
	}
	timeout := time.Duration(config.SourceFetchTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch source_url: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch source_url: HTTP %d", resp.StatusCode)
	}
	maxB := config.SourceFetchMaxBytes
	if maxB <= 0 {
		maxB = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxB+1))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > maxB {
		return "", fmt.Errorf("source exceeds max size (%d bytes)", maxB)
	}
	return string(body), nil
}

func validateTarget(ctx context.Context, u *url.URL) error {
	if config.AllowSourceFetchToPrivateNetworks {
		return nil
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("source_url resolves to a disallowed address")
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve source_url host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("source_url host has no addresses")
	}
	for _, ia := range ips {
		if isBlockedIP(ia.IP) {
			return fmt.Errorf("source_url host resolves to a disallowed address")
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 0 {
		return true
	}
	return false
}
