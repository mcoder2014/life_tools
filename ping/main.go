package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/miekg/dns"
)

var (
	typ  uint8 = 8
	code uint8 = 0

	timeout  int64 = 1000 // 耗时
	interval int64 = 1000 // 间隔
	size     int          // 大小
	i        int   = 1    // 循环次数

	SendCount int       = 0             // 发送次数
	RecvCount int       = 0             // 接收次数
	MaxTime   float64   = math.MinInt64 // 最大耗时
	MinTime   float64   = math.MaxInt64 // 最短耗时
	SumTime   float64   = 0             // 总计耗时
	AvgTime   float64   = 0
	Mdev      float64   = 0
	times     []float64 = make([]float64, i) // 记录每个请求耗时
)

type Statistics struct {
	startTime time.Time
	since     float64
	cname     string
}

// ICMP 序号不能乱
type ICMP struct {
	Type        uint8  // 类型
	Code        uint8  // 代码
	CheckSum    uint16 // 校验和
	ID          uint16 // ID
	SequenceNum uint16 // 序号
}

var statistics = &Statistics{}

func main() {
	log.SetFlags(log.Llongfile)
	flag.Parse()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c // 阻塞直到收到信号
		statistics.since = float64(time.Since(statistics.startTime).Nanoseconds())
		total()
		os.Exit(0)
	}()

	// 获取目标 IP
	domain := os.Args[len(os.Args)-1]
	cname, ips, err := MiekgResolveDomain(domain)
	statistics.cname = cname
	if err != nil {
		log.Println("domain name resolution failed: ", err)
		return
	}

	ip := ips[0].String()
	conn, err := net.DialTimeout("ip:icmp", ip, time.Duration(timeout)*time.Millisecond)

	if err != nil {
		log.Println(err.Error())
		return
	}
	defer conn.Close()

	fmt.Printf("PING %s (%s) %d(%d) bytes of data.\n", cname, ip, size, size+8+20)
	statistics.startTime = time.Now()

	ticker := time.NewTicker(time.Duration(interval) * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		sendICMP(conn, i, size)
		i++
	}
}

func checkSum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data)-1; i += 2 {
		sum += uint32(data[i])<<8 | uint32(data[i+1])
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1]) << 8
	}
	for (sum >> 16) > 0 {
		sum = (sum >> 16) + (sum & 0xFFFF)
	}
	return uint16(^sum)
}

func sendICMP(conn net.Conn, seq int, size int) error {
	// 构建请求
	icmp := &ICMP{
		Type:        typ,
		Code:        code,
		CheckSum:    uint16(0),
		ID:          uint16(seq),
		SequenceNum: uint16(seq),
	}

	// 将请求转为二进制流
	var buffer bytes.Buffer
	binary.Write(&buffer, binary.BigEndian, icmp)
	data := make([]byte, size)
	buffer.Write(data)
	data = buffer.Bytes()

	checkSum := checkSum(data)
	data[2] = byte(checkSum >> 8)
	data[3] = byte(checkSum)

	startTime := time.Now()
	_, err := conn.Write(data)
	if err != nil {
		return err
	}
	SendCount++
	buf := make([]byte, size+20+8+20)
	_, err = conn.Read(buf)
	if err != nil {
		return err
	}
	RecvCount++

	t := float64(time.Since(startTime).Nanoseconds()) / 1e6
	ip := fmt.Sprintf("%d.%d.%d.%d", buf[12], buf[13], buf[14], buf[15])
	fmt.Printf("%d bytes from %s: icmp_seq=%d time=%fms ttl=%d\n", len(data), ip, RecvCount, t, buf[8])
	MaxTime = math.Max(MaxTime, t)
	MinTime = math.Min(MinTime, t)
	SumTime += t
	times = append(times, t)
	return nil
}

func total() {
	mdev()
	t := float64(time.Since(statistics.startTime).Nanoseconds()) / 1e6
	fmt.Printf("\n--- %s ping statistics ---\n", statistics.cname)
	fmt.Printf("%d packets transmitted, %d received, %d packet loss, time %fms\n", SendCount, RecvCount, (i-1)*2-SendCount-RecvCount, t)
	fmt.Printf("rtt min/avg/max/mdev = %f/%f/%f/%f ms\n", MinTime, SumTime/float64(i), MaxTime, Mdev)
}

func mdev() {
	AvgTime = SumTime / float64(i)
	var sum float64 = 0
	for _, time := range times {
		sum += math.Pow(time-AvgTime, 2)
	}
	Mdev = math.Sqrt(sum / float64(i))
}

func MiekgResolveDomain(domain string) (string, []net.IP, error) {
	c := new(dns.Client)
	c.Timeout = 5 * time.Second

	m := new(dns.Msg)
	// dns.Fqdn(domain): 这个函数将域名转换为完全限定域名（Fully Qualified Domain Name, FQDN）格式。例如，如果 domain 是 "www.example.com"，dns.Fqdn(domain) 会返回 "www.example.com."（注意末尾的点）。这是 DNS 查询所需的标准格式。
	// dns.TypeA: 这指定了我们要查询的 DNS 记录类型。TypeA 表示我们要查询 IPv4 地址记录。如果我们想查询 IPv6 地址，可以使用 dns.TypeAAAA。
	m.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	// 223.5.5.5:53 是阿里的 DNS 服务器
	// 8.8.8.8:53 是 Google 的 DNS 服务器
	r, _, err := c.Exchange(m, "223.5.5.5:53")
	if err != nil {
		return "", nil, err
	}

	var cname string
	var ips []net.IP

	for _, ans := range r.Answer {
		switch record := ans.(type) {
		case *dns.CNAME:
			cname = strings.TrimSuffix(record.Target, ".")
		case *dns.A:
			ips = append(ips, record.A)
		}
	}
	return cname, ips, nil
}
