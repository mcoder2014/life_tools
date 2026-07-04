package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

const defaultAddr = "127.0.0.1:8787"

func main() {
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	codexHome := flag.String("codex-home", defaultCodexHome(), "Codex home directory")
	cachePath := flag.String("cache-path", defaultCachePath(), "SQLite cache path for historical session summaries")
	cacheWorkers := flag.Int("cache-workers", 2, "background workers for filling historical summary cache; 0 disables automatic fill")
	noCache := flag.Bool("no-cache", false, "disable SQLite summary cache")
	flag.Parse()

	if *noCache {
		*cachePath = ""
	}
	if *cacheWorkers < 0 {
		*cacheWorkers = 0
	}
	store := NewStoreWithCacheWorkers(*codexHome, *cachePath, *cacheWorkers)
	defer store.Close()
	fmt.Printf("codex_inspector reading data from %s\n", store.CodexHome)
	if store.CachePath != "" {
		fmt.Printf("codex_inspector caching historical summaries in %s\n", store.CachePath)
		if *cacheWorkers == 0 {
			fmt.Println("codex_inspector automatic cache fill disabled")
		} else {
			fmt.Printf("codex_inspector automatic cache fill workers: %d\n", *cacheWorkers)
		}
	}
	fmt.Printf("codex_inspector listening on http://%s\n", *addr)
	fmt.Println("privacy: this tool is read-only and should be bound to 127.0.0.1 unless you understand the data exposure risk")

	if err := http.ListenAndServe(*addr, NewServer(store)); err != nil {
		fmt.Fprintf(os.Stderr, "codex_inspector: listen failed: %v\n", err)
		os.Exit(1)
	}
}

func defaultCodexHome() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
}
