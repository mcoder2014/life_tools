package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

const defaultAddr = "0.0.0.0:8080"

func main() {
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	configPath := flag.String("config", "", "JSON config file path")
	flag.Parse()

	entries, err := loadEntries(*configPath, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "file_share: %v\n", err)
		os.Exit(1)
	}

	for _, entry := range entries {
		fmt.Printf("sharing [%d] %s -> %s\n", entry.ID, entry.Name, entry.Root)
	}
	fmt.Printf("file_share listening on http://%s\n", *addr)
	if err := http.ListenAndServe(*addr, NewServer(entries, os.Stdout)); err != nil {
		fmt.Fprintf(os.Stderr, "file_share: listen failed: %v\n", err)
		os.Exit(1)
	}
}

func loadEntries(configPath string, paths []string) ([]ShareEntry, error) {
	if configPath != "" && len(paths) > 0 {
		return nil, fmt.Errorf("-config and path arguments are mutually exclusive")
	}
	if configPath == "" {
		return BuildEntries(EntriesFromPaths(paths))
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	config, err := LoadConfig(content)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return BuildEntries(config.Entries)
}
