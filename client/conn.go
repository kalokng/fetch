package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"golang.org/x/net/websocket"
)

type funcConn func() (net.Conn, error)

func Connect(url, origin string) (net.Conn, error) {
	return websocket.Dial(url, "", origin)
}

func logConnect(w io.Writer, fn funcConn) funcConn {
	return func() (net.Conn, error) {
		conn, err := fn()
		if err != nil {
			fmt.Fprintf(w, "connect err: %s\n", err.Error())
		}
		return conn, err
	}
}

func HttpConnect(proxy, url_ string) (io.ReadWriteCloser, error) {
	p, err := net.Dial("tcp", proxy)
	if err != nil {
		return nil, err
	}

	turl, err := url.Parse(url_)
	if err != nil {
		return nil, err
	}

	req := http.Request{
		Method: "CONNECT",
		URL:    &url.URL{},
		Host:   turl.Host,
	}

	cc := httputil.NewProxyClientConn(p, nil)
	cc.Do(&req)
	if err != nil && err != httputil.ErrPersistEOF {
		return nil, err
	}

	rwc, _ := cc.Hijack()

	return rwc, nil
}

func ProxyDial(url_, protocol, origin string) (ws *websocket.Conn, err error) {
	if os.Getenv("HTTP_PROXY") == "" {
		return websocket.Dial(url_, protocol, origin)
	}

	purl, err := url.Parse(os.Getenv("HTTP_PROXY"))
	if err != nil {
		return nil, err
	}

	config, err := websocket.NewConfig(url_, origin)
	if err != nil {
		return nil, err
	}

	if protocol != "" {
		config.Protocol = []string{protocol}
	}

	client, err := HttpConnect(purl.Host, url_)
	if err != nil {
		return nil, err
	}

	return websocket.NewClient(config, client)
}
