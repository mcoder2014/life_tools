package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/json-iterator/go"
	"gopkg.in/yaml.v3"
)

const DefaultConfigPath = "/etc/life_tools/cq_ddns_client.json"

type Config struct {
	DDNSConfig []DomainConfig    `json:"ddns_config" yaml:"ddns_config"`
	Cloudflare *CloudflareConfig `json:"cloudflare" yaml:"cloudflare"`
}

type CloudflareConfig struct {
	APIToken string `json:"api_token" yaml:"api_token"`
	Zone     string `json:"zone" yaml:"zone"`
	Debug    bool   `json:"debug" yaml:"debug"`
	Proxy    string `json:"proxy" yaml:"proxy"`
}

type DomainConfig struct {
	Domain    string `json:"domain" yaml:"domain"`
	IPVersion string `json:"ip_version" yaml:"ip_version"`
}

func LoadConfig(path string) (Config, error) {
	var config Config
	content, err := os.ReadFile(path)
	if err != nil {
		return config, fmt.Errorf("read config: %w", err)
	}
	// The old home_client configuration remains usable with -conf path/to/config.yaml.
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(content, &config)
	default:
		err = jsoniter.Unmarshal(content, &config)
	}
	if err != nil {
		// JSON decoders may include the input, including credentials, in their errors.
		return Config{}, fmt.Errorf("invalid config syntax in %s", path)
	}
	if config.Cloudflare == nil || strings.TrimSpace(config.Cloudflare.APIToken) == "" || strings.TrimSpace(config.Cloudflare.Zone) == "" {
		return Config{}, fmt.Errorf("cloudflare.api_token and cloudflare.zone are required")
	}
	if len(config.DDNSConfig) == 0 {
		return Config{}, fmt.Errorf("ddns_config must contain at least one domain")
	}
	seen := make(map[[2]string]bool)
	for i, domain := range config.DDNSConfig {
		if strings.TrimSpace(domain.Domain) == "" || (domain.IPVersion != "ipv4" && domain.IPVersion != "ipv6") {
			return Config{}, fmt.Errorf("ddns_config[%d] requires domain and ip_version ipv4 or ipv6", i)
		}
		key := [2]string{strings.ToLower(strings.TrimSuffix(domain.Domain, ".")), domain.IPVersion}
		if seen[key] {
			return Config{}, fmt.Errorf("ddns_config[%d] repeats a domain and IP version", i)
		}
		seen[key] = true
	}
	return config, nil
}
