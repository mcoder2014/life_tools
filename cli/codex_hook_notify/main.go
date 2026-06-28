package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", DefaultConfigPath, "配置文件路径")
	flag.Parse()

	content, err := os.ReadFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex_hook_notify: read config failed: %v\n", err)
		return
	}
	config, err := LoadConfig(content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex_hook_notify: parse config failed: %v\n", err)
		return
	}

	_ = Run(context.Background(), RunOption{
		Config: config,
		Input:  os.Stdin,
		Stderr: os.Stderr,
		Sender: FeishuSender{},
		Logger: FileLogger{},
	})
}
