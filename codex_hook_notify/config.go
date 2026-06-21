package main

import "encoding/json"

const DefaultConfigPath = "/etc/life_tools/codex_hook_notify.json"

type Config struct {
	Routes []Route `json:"routes"`
}

type Route struct {
	Events                []string `json:"events"`
	FeishuCustomRobotURLs []string `json:"feishu_custom_robot_urls"`
}

func LoadConfig(content []byte) (Config, error) {
	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) MatchingURLs(eventName string) []string {
	var urls []string
	for _, route := range c.Routes {
		if !contains(route.Events, eventName) {
			continue
		}
		for _, url := range route.FeishuCustomRobotURLs {
			if url != "" {
				urls = append(urls, url)
			}
		}
	}
	return urls
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
