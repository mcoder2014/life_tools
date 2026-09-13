package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPFallbackAndValidation(t *testing.T) {
	var visited []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visited = append(visited, r.URL.Path)
		switch r.URL.Path {
		case "/error":
			w.WriteHeader(http.StatusBadGateway)
		case "/html":
			_, _ = w.Write([]byte("<html>unavailable</html>"))
		case "/v6":
			_, _ = w.Write([]byte("2001:db8::1\n"))
		default:
			_, _ = w.Write([]byte(" 203.0.113.10\n"))
		}
	}))
	defer server.Close()
	urls := []string{server.URL + "/error", server.URL + "/html", server.URL + "/v6", server.URL + "/v4"}
	ip, err := getIPAddress(context.Background(), server.Client(), urls, "ipv4")
	require.NoError(t, err)
	require.Equal(t, "203.0.113.10", ip)
	require.Equal(t, []string{"/error", "/html", "/v6", "/v4"}, visited)
	_, err = getIPAddress(context.Background(), server.Client(), urls[:3], "ipv4")
	require.Error(t, err)
	ip, err = getIPAddress(context.Background(), server.Client(), []string{server.URL + "/v6"}, "ipv6")
	require.NoError(t, err)
	require.Equal(t, "2001:db8::1", ip)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = getIPAddress(ctx, server.Client(), urls, "ipv4")
	require.ErrorIs(t, err, context.Canceled)
}
