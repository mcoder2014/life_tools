package main

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Info struct {
	Cname string
	Ip    []net.IP
}

func resolveDomain(domain string) (*Info, error) {
	records, err := net.LookupIP(domain)
	if err != nil {
		return nil, fmt.Errorf("查找IP时出错: %v", err)
	}

	cname, err := net.LookupCNAME(domain)
	if err != nil {
		// 如果找不到CNAME，就使用原始域名
		cname = domain + "."
	}

	return &Info{Cname: cname, Ip: records}, nil
}
func ResolveDomainWithTimeout(domain string, timeout time.Duration) (*Info, error) {
	startTime := time.Now()
	resultChan := make(chan *Info, 1)
	errorChan := make(chan error, 1)

	for {
		go func() {
			info, err := resolveDomain(domain)
			if err != nil {
				errorChan <- err
			} else {
				resultChan <- info
			}
		}()
		select {
		case result := <-resultChan:
			fmt.Println(result, "ss")
			if result.Cname == domain+"." {
				return result, nil
			}
			// CNAME 不匹配，继续解析新的域名
			domain = strings.TrimSuffix(result.Cname, ".")
		case err := <-errorChan:
			return nil, err
		case <-time.After(timeout - time.Since(startTime)):
			return nil, fmt.Errorf("域名解析超时")
		}

		// 检查是否超时
		if time.Since(startTime) >= timeout {
			return nil, fmt.Errorf("域名解析超时")
		}
	}
}

func ResolveDomain(initialDomain string) (string, []net.IP, error) {
	domain := initialDomain
	info, err := ResolveDomainWithTimeout(domain, 5*time.Second)
	if err != nil {
		return "", nil, err
	}
	domain = info.Cname[:len(info.Cname)-1] // 移除末尾的点
	return domain, info.Ip, nil
}
