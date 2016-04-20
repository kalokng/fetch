package main

import (
	"fmt"
	"net"

	"net/http"
	_ "net/http/pprof"

	"golang.org/x/net/websocket"
)

func Connect(url, origin string) (net.Conn, error) {
	return websocket.Dial(url, "", origin)
}

func main() {
	origin := "http://localhost/"
	url := "ws://localhost:12345/proxy"
	pool := NewConnPool(func() (net.Conn, error) {
		fmt.Println("     New connection")
		conn, err := Connect(url, origin)
		if err != nil {
			return nil, err
		}
		return conn, nil
	})

	host := ":8282"
	fmt.Println("Start listening", host)
	//http.Handle("/", Tunnel(pool))
	err := http.ListenAndServe(host, Tunnel(pool))
	if err != nil {
		panic(err)
	}
}
