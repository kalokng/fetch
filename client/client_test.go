package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/kalokng/fetch"
)

func TestWeb(t *testing.T) {
	resp, err := http.Get("https://toy-fands.rhcloud.com/web")
	if err != nil {
		panic(err)
	}
	resp.Write(os.Stdout)
}

func TestWsecho(t *testing.T) {
	var n int
	url := "ws://toy-fands.rhcloud.com:8000/echo"
	conn, err := ProxyDial(url, "", url)
	if err != nil {
		panic(err)
	}
	fmt.Println("conn done")
	b := make([]byte, 256, 512)
	for i := range b {
		b[i] = 255 - byte(i)
	}
	b = append(b, []byte("世界")...)

	ew := fetch.NewEncoder(conn)

	go func() {
		n, err := ew.Write(b)
		fmt.Println(n, err)
	}()

	time.Sleep(1e9)

	msg := make([]byte, 1024)
	er := fetch.NewDecoder(conn)
	n, err = er.Read(msg)
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
	fmt.Printf("%[1]s\n% [1]x\n", msg[:n])

	if n != len(b) {
		t.Error("Wrong count")
		return
	}
	for i, v := range b {
		if msg[i] != v {
			t.Errorf("Wrong @ %d:%X %X", i, v, msg[i])
			return
		}
	}
}

func TestEcho(t *testing.T) {
	url := "http://toy2-fands.rhcloud.com/echo3"
	conn, err := ProxyHTTP(url)
	if err != nil {
		panic(err)
	}
	fmt.Println("conn done")
	b := make([]byte, 256, 512)
	for i := range b {
		b[i] = 255 - byte(i)
	}
	b = append(b, []byte("世界")...)

	go func() {
		n, err := conn.Write(b)
		fmt.Println(n, err)
	}()

	time.Sleep(1e9)

	msg := make([]byte, 1024)
	n, err := conn.Read(msg)
	if err != nil {
		panic(err)
	}
	de := make([]byte, 512)
	n, err = hex.Decode(de, msg[:n])
	if err != nil {
		panic(err)
	}
	fmt.Printf("%[1]s\n% [1]x\n", de[:n])

	if n != len(b) {
		t.Error("Wrong count")
		return
	}
	for i, v := range b {
		if de[i] != v {
			t.Errorf("Wrong @ %d:%X %X", i, v, de[i])
			return
		}
	}
}
