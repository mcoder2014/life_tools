package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

type Client struct {
	config       Config
	api          *cloudflare.API
	ipClient     *http.Client
	ipv4Services []string
	ipv6Services []string
}

func NewClient(config Config) (*Client, error) {
	httpClient := &http.Client{Timeout: 20 * time.Second}
	if config.Cloudflare.Proxy != "" {
		dialer, err := proxy.SOCKS5("tcp", config.Cloudflare.Proxy, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("create Cloudflare SOCKS5 proxy: %w", err)
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, fmt.Errorf("Cloudflare SOCKS5 proxy does not support context")
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.DialContext = contextDialer.DialContext
		httpClient.Transport = transport
	}
	api, err := cloudflare.NewWithAPIToken(config.Cloudflare.APIToken, cloudflare.HTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	// Do not enable SDK HTTP dumps: they include the Authorization header.
	// Legacy debug controls application logging in main instead.
	return &Client{
		config: config, api: api, ipClient: &http.Client{Timeout: 5 * time.Second},
		ipv4Services: []string{"https://4.ipw.cn", "https://api4.ipify.org", "https://ipv4.icanhazip.com", "https://v4.ident.me"},
		ipv6Services: []string{"https://6.ipw.cn", "https://api6.ipify.org", "https://ipv6.icanhazip.com", "https://v6.ident.me"},
	}, nil
}

func (c *Client) Refresh(ctx context.Context, dryRun bool) (err error) {
	defer recoverCloudflareError(ctx, &err)
	logrus.Infof("DDNS refresh start: domains=%d dry_run=%t", len(c.config.DDNSConfig), dryRun)
	records, err := c.api.DNSRecords(ctx, c.config.Cloudflare.Zone, cloudflare.DNSRecord{})
	if err != nil {
		return fmt.Errorf("list DNS records: %w", err)
	}
	// A and AAAA may share a name. Multiple records of the same type are ambiguous.
	byNameAndType := make(map[[2]string][]cloudflare.DNSRecord)
	for _, record := range records {
		key := [2]string{strings.ToLower(strings.TrimSuffix(record.Name, ".")), record.Type}
		byNameAndType[key] = append(byNameAndType[key], record)
	}
	failed := 0
	for _, domain := range c.config.DDNSConfig {
		if err := ctx.Err(); err != nil {
			return err
		}
		typeName, services := "A", c.ipv4Services
		if domain.IPVersion == "ipv6" {
			typeName, services = "AAAA", c.ipv6Services
		}
		key := [2]string{strings.ToLower(strings.TrimSuffix(domain.Domain, ".")), typeName}
		ip, err := getIPAddress(ctx, c.ipClient, services, domain.IPVersion)
		if err == nil {
			err = c.syncRecord(ctx, domain.Domain, typeName, ip, byNameAndType[key], dryRun)
		}
		if err != nil {
			failed++
			logrus.WithError(err).Errorf("DDNS refresh failed: domain=%s type=%s", domain.Domain, typeName)
		}
	}
	if failed != 0 {
		return fmt.Errorf("DDNS refresh failed for %d of %d domains", failed, len(c.config.DDNSConfig))
	}
	logrus.Info("DDNS refresh complete")
	return nil
}

func (c *Client) syncRecord(ctx context.Context, domain, typeName, ip string, records []cloudflare.DNSRecord, dryRun bool) (err error) {
	defer recoverCloudflareError(ctx, &err)
	if len(records) > 1 {
		return fmt.Errorf("multiple DNS records match domain and type; refusing ambiguous update")
	}
	proxied := false
	record := cloudflare.DNSRecord{Name: domain, Type: typeName, Content: ip, TTL: 300, Proxied: &proxied}
	action := "create"
	if len(records) == 1 {
		record = records[0]
		if net.ParseIP(ip).Equal(net.ParseIP(record.Content)) {
			logrus.Infof("domain=%s type=%s ip=%s unchanged, skip update", domain, typeName, ip)
			return nil
		}
		record.Content = ip
		action = "update"
	}
	if dryRun {
		logrus.Infof("dry-run: would %s domain=%s type=%s ip=%s", action, domain, typeName, ip)
		return nil
	}
	if action == "create" {
		resp, err := c.api.CreateDNSRecord(ctx, c.config.Cloudflare.Zone, record)
		if err != nil {
			return err
		}
		if resp == nil || !resp.Success {
			return fmt.Errorf("Cloudflare did not confirm DNS record creation")
		}
	} else if err := c.api.UpdateDNSRecord(ctx, c.config.Cloudflare.Zone, record.ID, record); err != nil {
		return err
	}
	logrus.Infof("%s success: domain=%s type=%s ip=%s", action, domain, typeName, ip)
	return nil
}

// cloudflare-go v0.47.1 dereferences a nil response on transport errors.
// Keep the old client's recovery boundary, returning errors so one failed write
// does not stop other domains and the daemon can retry on its next cycle.
func recoverCloudflareError(ctx context.Context, err *error) {
	if recovered := recover(); recovered != nil {
		if ctx.Err() != nil {
			*err = ctx.Err()
		} else {
			*err = fmt.Errorf("Cloudflare SDK request panicked; check network and proxy connectivity")
		}
	}
}
