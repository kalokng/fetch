package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"net/http"
	_ "net/http/pprof"
)

func main() {
	host := "localhost"
	port := "8000"
	if len(os.Args) >= 2 {
		l := strings.Split(os.Args[1], ":")
		host = l[0]
		if len(l) > 1 {
			port = l[1]
		}
	}
	fmt.Printf("Connect to %s:%s\n", host, port)

	origin := "http://" + host + "/"
	url := "ws://" + host + ":" + port + "/proxy"

	genConn := func() (net.Conn, error) {
		conn, err := ProxyDial(url, "", origin)
		if err != nil {
			return nil, err
		}
		return conn, nil
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
