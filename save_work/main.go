package main

import (
	"flag"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

func main() {
	p := getParam()
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	if p.Quiet {
		logrus.SetLevel(logrus.PanicLevel)
	}
	logrus.Infof("PWD:%v param:%v", os.Getenv("PWD"), *p)

	// 从配置文件中读取真实的关键词，防止敏感关键词被提交到 github
	config, err := GetConfig(p.ConfigPath)
	if err != nil {
		logrus.WithError(err).Errorf("load config failed. please set config file %v", p.ConfigPath)
		os.Exit(1)
	}
	logrus.Infof("load config: %+v", *config)

	content, err := ReceiveContents()
	if err != nil {
		logrus.Errorf("Receive contents error: %v", err)
		return
	}

	lines := PrepareContents(string(content))
	res, _ := ExistKeywords(lines, config.KeyWords, WithParallel(p.Parallel))
	if res {
		logrus.Infof("exist keywords")
		os.Exit(1)
	} else {
		logrus.Infof("not exist keywords")
	}
}

func ReceiveContents() ([]byte, error) {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return content, nil
}

// PrepareContents 此处用于 git diff 新增内容的过滤
func PrepareContents(contents string) []string {
	lines := strings.Split(contents, "\n")

	var res []string
	for _, line := range lines {
		if len(line) > 0 && line[0] == '+' {
			res = append(res, line)
		}
	}
	return res
}

type param struct {
	ConfigPath string
	Parallel   int
	Quiet      bool
}

func getParam() *param {
	var p param

	flag.IntVar(&p.Parallel, "parallel", runtime.NumCPU(), "并行执行的粒度")
	flag.StringVar(&p.ConfigPath, "config", "/etc/life_tools/check_keywords.json", "配置文件路径")
	flag.BoolVar(&p.Quiet, "quiet", false, "是否静默模式")
	flag.Parse()
	return &p
}
