package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/webdav"
)

func main() {
	var addr *string
	var path *string
	addr = flag.String("addr", ":8080", "") // listen端口，默认8080
	path = flag.String("path", ".", "")     // 文件路径，默认当前目录
	flag.Parse()

	fmt.Println("addr=", *addr, ", path=", *path) // 在控制台输出配置
	webdavWithAuth(*addr, *path)
}

func webdavWithoutAuth(addr, path string) {
	http.ListenAndServe(addr, &webdav.Handler{
		FileSystem: webdav.Dir(path),
		LockSystem: webdav.NewMemLS(),
	})
}

func webdavWithAuth(addr, path string) {
	fs := &webdav.Handler{
		FileSystem: webdav.Dir(path),
		LockSystem: webdav.NewMemLS(),
	}
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Printf("req url %v\n", req.URL)
		for key, value := range req.Header {
			fmt.Printf("req header %s:%s\n", key, value)
		}

		// 获取用户名/密码
		username, password, ok := req.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `basic realm="Restricted"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		logrus.Infof("user:%v password:%v", username, password)

		// 验证用户名/密码
		if username != "user" || password != "123456" {
			http.Error(w, "WebDAV: need authorized!", http.StatusUnauthorized)
			logrus.Errorf("WebDAV: need authorized!")
			return
		}
		logrus.Infof("WebDAV: authorized success!")
		fs.ServeHTTP(w, req)
	})
	http.ListenAndServe(addr, nil)
}
