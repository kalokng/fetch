package main

import (
	"fmt"
	"net"

	"golang.org/x/net/websocket"
)

func Connect(url, origin string) (net.Conn, error) {
	return websocket.Dial(url, "", origin)
}

func main() {
	origin := "http://localhost/"
	url := "ws://localhost:12345/web"
	pool := NewConnPool(func() (net.Conn, error) {
		fmt.Println("New connection")
		conn, err := Connect(url, origin)
		if err != nil {
			return nil, err
		}
		return conn, nil
	})

	err := Tunnel(":8282", pool)
	if err != nil {
		panic(err)
	}
}
