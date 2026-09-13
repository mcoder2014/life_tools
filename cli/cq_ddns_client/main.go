package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", DefaultConfigPath, "配置文件路径（JSON，兼容 .yaml/.yml）")
	flag.StringVar(&configPath, "conf", DefaultConfigPath, "旧 home_client 参数，等价于 -config")
	once := flag.Bool("once", false, "刷新一次后退出；失败时返回非零退出码")
	dryRun := flag.Bool("dry-run", false, "只读预检一次，显示计划变更，不写 DNS")
	flag.Parse()
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	config, err := LoadConfig(configPath)
	if err != nil {
		logrus.WithError(err).Error("load config failed")
		os.Exit(1)
	}
	if config.Cloudflare.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	client, err := NewClient(config)
	if err != nil {
		logrus.WithError(err).Error("initialize DDNS client failed")
		os.Exit(1)
	}
	logrus.Infof("cq_ddns_client started: config=%s domains=%d interval=120s", configPath, len(config.DDNSConfig))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, client, *once || *dryRun, *dryRun); err != nil {
		logrus.WithError(err).Error("DDNS run failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, client *Client, once, dryRun bool) error {
	ticker := time.NewTicker(120 * time.Second)
	defer ticker.Stop()
	for {
		err := client.Refresh(ctx, dryRun)
		if ctx.Err() != nil {
			logrus.Info("cq_ddns_client stopped")
			return nil
		}
		if once {
			return err
		}
		if err != nil {
			logrus.WithError(err).Error("DDNS cycle failed; retrying on next tick")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
