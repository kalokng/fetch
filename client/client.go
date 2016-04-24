package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/kalokng/fetch"

	"net/http"
	_ "net/http/pprof"
)

func main() {
	host := "localhost"
	port := "8000"
	proto := "https"
	if len(os.Args) >= 2 {
		sch := strings.SplitN(os.Args[1], "://", 2)
		if len(sch) > 1 {
			proto = sch[0]
		}
		addr := sch[len(sch)-1]
		l := strings.Split(addr, ":")
		host = l[0]
		if len(l) > 1 {
			port = l[1]
		}
	}

	var origin, url string
	switch proto {
	case "http":
		origin = "http://" + host + "/"
		url = "ws://" + host + ":" + port + "/proxy"
	case "https":
		origin = "https://" + host + "/"
		url = "wss://" + host + ":" + port + "/proxy"
	default:
		fmt.Println("Unknown protocol")
		return
	}

	fmt.Printf("Connect to %s:%s\n", host, port)

	genConn := func() (net.Conn, error) {
		conn, err := ProxyDial(url, "", origin)
		if err != nil {
			return nil, err
		}
		return fetch.NewUtf8Conn(conn), nil
	}
	genConn = logConnect(os.Stdout, genConn)

	pool := NewConnPool(genConn)

	proxy := ":8282"
	fmt.Println("Start listening", proxy)
	//http.Handle("/", Tunnel(pool))
	err := http.ListenAndServe(proxy, Tunnel(pool))
	if err != nil {
		panic(err)
	}
}
