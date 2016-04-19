package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"

	"golang.org/x/net/websocket"
)

var echoWs = websocket.Handler(func(ws *websocket.Conn) {
	os.Stdout.Write([]byte("Start ECHO"))
	defer os.Stdout.Write([]byte("End ECHO"))
	r := io.TeeReader(ws, os.Stdout)
	io.Copy(ws, r)
})

func EchoServer(w http.ResponseWriter, r *http.Request) {
	echoWs.ServeHTTP(w, r)
}

func WebServer(ws *websocket.Conn) {
	os.Stdout.Write([]byte("Start WEB"))
	defer os.Stdout.Write([]byte("End WEB"))
	w := io.MultiWriter(ws, os.Stdout)

	resp, err := http.Get("http://httpbin.org/ip")
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	resp.Write(w)
}

func dispatcher(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		//h := websocket.Handler(WebServer)
		//h.ServeHTTP(w, r)

		// Wrap it with as ResponseWriter, and call ServeHTTP
		websocket.Handler(func(ws *websocket.Conn) {
			fmt.Println("Start ws")
			defer fmt.Println("End ws")
			req, err := http.ReadRequest(bufio.NewReader(ws))
			if err != nil {
				io.WriteString(ws, "HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\n400 Bad Request")
				return
			}
			//b, _ := httputil.DumpRequestOut(req, true)
			//os.Stdout.Write(b)
			fmt.Println("req.RequestURI", req.RequestURI)
			req.RequestURI = ""
			fmt.Println("req.URL.Scheme", req.URL.Scheme)
			req.URL.Scheme = "http"
			fmt.Println("req.URL.Host", req.URL.Host)
			req.URL.Host = req.Host

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println(err)
				io.WriteString(ws, "HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\n400 Bad Request: "+err.Error())
				return
			}
			resp.Write(ws)
		}).ServeHTTP(w, r)
	})
}

func main() {
	http.HandleFunc("/echo", EchoServer)
	//proxy := NewProxyListener(nil)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Hello world")
		fmt.Fprintf(w, "Hello world!")
	})
	err := http.ListenAndServe(":12345", dispatcher(h))
	if err != nil {
		panic(err)
	}
}
