package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// Query services in order, accepting only an address of the requested family.
// A service error page or a wrong-family response must never reach Cloudflare.
func getIPAddress(ctx context.Context, client *http.Client, urls []string, version string) (string, error) {
	var lastErr error
	for _, url := range urls {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err == nil {
			var body []byte
			body, err = io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("IP service returned HTTP %d", resp.StatusCode)
			}
			if err == nil {
				address := strings.TrimSpace(string(body))
				ip := net.ParseIP(address)
				if ip != nil && ((version == "ipv4" && ip.To4() != nil) || (version == "ipv6" && ip.To4() == nil)) {
					return address, nil
				}
				err = fmt.Errorf("IP service did not return a valid %s address", version)
			}
		}
		logrus.WithError(err).Warnf("IP service %s failed, trying next", url)
		lastErr = err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("all %s query services failed: %w", version, lastErr)
}
