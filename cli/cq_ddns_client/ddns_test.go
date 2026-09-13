package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cloudflare/cloudflare-go"
	"github.com/stretchr/testify/require"
)

// Exercise the real Cloudflare SDK over loopback; no token or public DNS is used.
func TestRefreshDNSRecords(t *testing.T) {
	for _, mode := range []string{"create", "update", "unchanged", "dry-run", "list error", "write error", "ip error", "duplicate", "list transport error", "write transport error"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var writes []cloudflare.DNSRecord
			var paths []string
			proxied := true
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/ipv4" || r.URL.Path == "/ipv6" {
					if mode == "ip error" && r.URL.Path == "/ipv4" {
						w.WriteHeader(http.StatusBadGateway)
						return
					}
					ip := "203.0.113.10"
					if r.URL.Path == "/ipv6" {
						ip = "2001:db8::10"
					}
					_, _ = w.Write([]byte(ip))
					return
				}
				if mode == "list transport error" && r.Method == http.MethodGet {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					conn.Close()
					return
				}
				if r.Method == http.MethodGet {
					if mode == "list error" {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"denied"}]}`))
						return
					}
					records := []cloudflare.DNSRecord{{ID: "txt", Name: "pi.example.org", Type: "TXT", Content: "keep"}}
					if mode != "create" {
						v4, v6 := "203.0.113.1", "2001:db8::1"
						if mode == "unchanged" {
							v4, v6 = "203.0.113.10", "2001:0db8:0:0:0:0:0:10"
						}
						records = append(records,
							cloudflare.DNSRecord{ID: "v4", Name: "pi.example.org", Type: "A", Content: v4, TTL: 600, Proxied: &proxied},
							cloudflare.DNSRecord{ID: "v6", Name: "pi.example.org", Type: "AAAA", Content: v6, TTL: 900, Proxied: &proxied})
					}
					if mode == "duplicate" {
						records = append(records, records[1])
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "result": records, "result_info": map[string]int{"page": 1, "total_pages": 1}})
					return
				}
				var record cloudflare.DNSRecord
				if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
					t.Error(err)
				}
				writes = append(writes, record)
				paths = append(paths, r.Method+" "+r.URL.Path)
				if mode == "write transport error" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					conn.Close()
					return
				}
				if mode == "write error" {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":10000,"message":"denied"}]}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "result": record})
			}))
			defer server.Close()
			api, err := cloudflare.NewWithAPIToken("test-token", cloudflare.BaseURL(server.URL), cloudflare.HTTPClient(server.Client()), cloudflare.UsingRetryPolicy(0, 0, 0))
			require.NoError(t, err)
			client := &Client{
				config: Config{Cloudflare: &CloudflareConfig{Zone: "test-zone"}, DDNSConfig: []DomainConfig{{Domain: "pi.example.org", IPVersion: "ipv4"}, {Domain: "pi.example.org", IPVersion: "ipv6"}}},
				api:    api, ipClient: server.Client(), ipv4Services: []string{server.URL + "/ipv4"}, ipv6Services: []string{server.URL + "/ipv6"},
			}
			require.NotPanics(t, func() {
				err = client.Refresh(context.Background(), mode == "dry-run")
			})
			mu.Lock()
			defer mu.Unlock()
			if mode == "list error" || mode == "write error" || mode == "ip error" || mode == "duplicate" || mode == "list transport error" || mode == "write transport error" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			switch mode {
			case "unchanged", "dry-run", "list error", "list transport error":
				require.Empty(t, writes)
			case "create":
				require.Len(t, writes, 2)
				require.Equal(t, []string{"POST /zones/test-zone/dns_records", "POST /zones/test-zone/dns_records"}, paths)
				require.Equal(t, "A", writes[0].Type)
				require.Equal(t, "AAAA", writes[1].Type)
				require.Equal(t, 300, writes[0].TTL)
				require.NotNil(t, writes[0].Proxied)
				require.False(t, *writes[0].Proxied)
			case "update", "write error", "write transport error":
				require.Equal(t, []string{"PATCH /zones/test-zone/dns_records/v4", "PATCH /zones/test-zone/dns_records/v6"}, paths)
				require.Equal(t, 600, writes[0].TTL)
				require.True(t, *writes[0].Proxied)
				require.Equal(t, "203.0.113.10", writes[0].Content)
				require.Equal(t, "2001:db8::10", writes[1].Content)
			case "ip error", "duplicate":
				require.Equal(t, []string{"PATCH /zones/test-zone/dns_records/v6"}, paths)
			}
		})
	}
}

func TestCloudflareCancellationAndNextRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cancel/zones/test-zone/dns_records" {
			cancel()
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{Cloudflare: &CloudflareConfig{APIToken: "test-token", Zone: "test-zone", Debug: true}})
	require.NoError(t, err)
	require.False(t, client.api.Debug, "legacy debug must never expose the Authorization header")
	client.api.BaseURL = server.URL + "/cancel"
	// Direct cancellation must propagate rather than crash in the pinned SDK.
	require.NotPanics(t, func() {
		err = client.Refresh(ctx, false)
	})
	require.ErrorIs(t, err, context.Canceled)
	client.api.BaseURL = server.URL
	require.NoError(t, client.Refresh(context.Background(), false))
	require.NoError(t, run(ctx, client, true, false), "shutdown is a normal exit")
}
