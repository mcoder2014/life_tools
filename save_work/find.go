package main

import (
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func ExistKeywords(lines []string, keywords []string, opts ...CheckKeywordsOption) (bool, error) {

	opt := defaultCheckKeywordsOption()
	for _, o := range opts {
		o(opt)
	}
	logrus.Infof("CheckExistKeywords option %+v", *opt)

	var result atomic.Value
	result.Store(false)

	batchLines := splitLines(lines, opt.parallel)
	// 并行计算
	var eg = errgroup.Group{}
	for _, eachLines := range batchLines {
		localEachLines := eachLines
		eg.Go(func() error {
			if checkKeywordsContains(localEachLines, keywords) {
				result.Store(true)
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return false, err
	}
	return result.Load().(bool), nil
}

func splitLines(lines []string, parallel int) [][]string {
	var res [][]string
	var linesPerParallel = len(lines) / parallel
	for i := 0; i < parallel; i++ {
		var start = i * linesPerParallel
		var end = start + linesPerParallel
		if i == parallel-1 {
			end = len(lines)
		}
		res = append(res, lines[start:end])
	}
	return res
}

func checkKeywordsContains(lines []string, keywords []string) bool {
	for _, line := range lines {
		for _, keyword := range keywords {
			if strings.Contains(line, keyword) {
				logrus.Infof("hit line:%v keyword:%v", line, keyword)
				return true
			}
		}
	}
	return false
}

type checkKeywordsOption struct {
	// 并发程度
	parallel int
}

func defaultCheckKeywordsOption() *checkKeywordsOption {
	return &checkKeywordsOption{
		parallel: 1,
	}
}

type CheckKeywordsOption func(*checkKeywordsOption)

func WithParallel(parallel int) CheckKeywordsOption {
	return func(o *checkKeywordsOption) {
		if parallel <= 0 {
			return
		}
		o.parallel = parallel
	}
}
