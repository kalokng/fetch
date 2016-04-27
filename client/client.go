package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/kalokng/fetch"

	"golang.org/x/net/websocket"

	"net/http"
	_ "net/http/pprof"
)

var proxyUrl string
var hostUrl string
var localPort string

func getRemoteProxy() string {
	e := os.Getenv("REMOTE_PROXY")
	if e == "" {
		return "http://localhost:8000"
	}
	return e
}

func init() {
	flag.StringVar(&localPort, "port", "8282", "the port this server going to listen")
	flag.StringVar(&hostUrl, "host", getRemoteProxy(), "Address of Remote server,\ndefault is $REMOTE_PROXY if set")
	flag.StringVar(&proxyUrl, "proxy", os.Getenv("HTTP_PROXY"), "Address of HTTP proxy server, empty if no proxy.\nDefault is $HTTP_PROXY if set")
}

func main() {
	flag.Parse()
	host := "localhost"
	port := "8000"
	proto := "https"

	sch := strings.SplitN(hostUrl, "://", 2)
	if len(sch) > 1 {
		proto = sch[0]
	}
	addr := sch[len(sch)-1]
	l := strings.Split(addr, ":")
	host = l[0]
	if len(l) > 1 {
		port = l[1]
	}

	var origin, url string
	switch proto {
	case "http":
		origin = "http://" + host + "/"
		url = "ws://" + host + ":" + port + "/p"
	case "https":
		origin = "https://" + host + "/"
		url = "wss://" + host + ":" + port + "/p"
	default:
		fmt.Println("Unknown protocol")
		return
	}

	fmt.Printf("Connect to %s:%s\n", host, port)
	if proxyUrl == "" {
		fmt.Println("Without http proxy")
	} else {
		fmt.Printf("Wtih http proxy %s\n", proxyUrl)
	}

	genConn := func() (net.Conn, error) {
		conn, err := ProxyDial(url, "", origin)
		if err != nil {
			return nil, err
		}
		conn.PayloadType = websocket.BinaryFrame
		return fetch.NewClientConn(conn, 0x56), nil
	}
	genConn = logConnect(genConn)

	pool := NewConnPool(genConn)

	fmt.Println("Start listening", ":"+localPort)
	//http.Handle("/", Tunnel(pool))
	err := http.ListenAndServe(":"+localPort, LogHandler(Tunnel(pool)))
	if err != nil {
		panic(err)
	}
}
