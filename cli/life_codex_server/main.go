package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	lifecodex "life_tools/internal/life_codex"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "audit-clean" {
		runAuditClean(os.Args[2:])
		return
	}
	runServe(os.Args[1:])
}

func runServe(args []string) {
	flags := flag.NewFlagSet("life_codex_server", flag.ExitOnError)
	configPath := flags.String("config", lifecodex.DefaultServerConfigPath, "server config path")
	addr := flags.String("addr", "", "listen address override")
	dataDir := flags.String("data-dir", "", "data directory override")
	auditDir := flags.String("audit-dir", "", "audit directory override")
	webRoot := flags.String("web-root", "", "web root override")
	adminToken := flags.String("admin-token", "", "admin token override")
	flags.Parse(args)

	config, err := loadServerConfigForCLI(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if *addr != "" {
		config.Addr = *addr
	}
	if *dataDir != "" {
		config.DataDir = *dataDir
	}
	if *auditDir != "" {
		config.AuditDir = *auditDir
	}
	if *webRoot != "" {
		config.WebRoot = *webRoot
	}
	if *adminToken != "" {
		config.AdminToken = *adminToken
	}
	if config.AdminToken == "" {
		log.Fatal("admin_token is required")
	}
	store, err := lifecodex.NewStateStore(config)
	if err != nil {
		log.Fatal(err)
	}
	server := lifecodex.NewHTTPServer(config, store)
	httpServer := &http.Server{Addr: config.Addr, Handler: server.Handler()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	log.Printf("life_codex_server listening on http://%s", config.Addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func runAuditClean(args []string) {
	flags := flag.NewFlagSet("audit-clean", flag.ExitOnError)
	configPath := flags.String("config", lifecodex.DefaultServerConfigPath, "server config path")
	beforeValue := flags.String("before", "", "delete audit files before YYYY-MM-DD")
	confirm := flags.Bool("confirm", false, "confirm deletion")
	flags.Parse(args)
	if !*confirm {
		log.Fatal("audit-clean requires --confirm")
	}
	before, err := time.Parse("2006-01-02", *beforeValue)
	if err != nil {
		log.Fatal("--before must be YYYY-MM-DD")
	}
	config, err := loadServerConfigForCLI(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	removed, err := lifecodex.ClearAuditBefore(config.AuditDir, before)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("removed %d audit file(s)\n", removed)
}

func loadServerConfigForCLI(path string) (lifecodex.ServerConfig, error) {
	config, err := lifecodex.LoadServerConfig(path)
	if err == nil {
		return config, nil
	}
	if path != "" && os.IsNotExist(err) {
		return lifecodex.DefaultServerConfig(), nil
	}
	return config, err
}
