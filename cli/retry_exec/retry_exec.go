package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mcoder2014/go_utils/log"
)

func main() {
	runtime.GOMAXPROCS(1)
	ctx := context.Background()
	opt := getOption()

	// 初始化日志组件
	_ = log.Init(&log.MyLogConfig{
		SavePath: filepath.Join(opt.Log, "retry_exec.log"),
	})

	commands := formatArgs(os.Args[1:])
	log.Ctx(ctx).Infof("args: %v", commands)

	retry := NewRetry(commands, opt)
	err := retry.Try(ctx)
	if err != nil {
		log.Ctx(ctx).WithError(err).Errorf("retry_exec execute failed, commands: %v", commands)
		os.Exit(retry.ExitCode)
	}
}

func getOption() *ExecOption {
	var configFile string
	flag.StringVar(&configFile, "config", "/etc/life_tools/retry_exec.json", "配置文件路径")
	help := flag.Bool("help", false, "帮助")
	flag.Parse()

	if *help || len(os.Args) == 1 {
		printHelp()
		os.Exit(0)
	}

	if len(configFile) == 0 {
		printHelp()
		os.Exit(1)
	}
	fmt.Printf("retry_exec config file: %s\n", configFile)

	content, err := os.ReadFile(configFile)
	if err != nil {
		fmt.Printf("read config file failed, err=%+v\n", err)
		os.Exit(1)
	}

	var config ExecOption
	err = json.Unmarshal(content, &config)
	if err != nil {
		fmt.Printf("parse config file failed, err=%+v\n", err)
		os.Exit(1)
	}

	return &config
}

func formatArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	if (args[0] == "--config" || args[0] == "-config") && len(args) > 2 {
		return args[2:]
	}

	return args
}

func printHelp() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(flag.CommandLine.Output(), `
使用格式如下

1. 基本格式如下
retry_exec [your command] [your command args] [your command args] ...

2. 指定配置文件使用时的格式如下
retry_exec --config [your config file] [your command] [your command args] [your command args]...

`)
	flag.PrintDefaults()
}
