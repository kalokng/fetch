package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/kalokng/fetch"

	"golang.org/x/net/websocket"

	"net/http"
	"net/url"
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
	flag.StringVar(&hostUrl, "host", getRemoteProxy(), "Address of Remote server, $REMOTE_PROXY if set")
	flag.StringVar(&proxyUrl, "proxy", os.Getenv("HTTP_PROXY"), "Address of HTTP proxy server, $HTTP_PROXY if set")
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

	var origin, pUrl string
	switch proto {
	case "http":
		origin = "http://" + host + "/"
		pUrl = "ws://" + host + ":" + port + "/p"
	case "https":
		origin = "https://" + host + "/"
		pUrl = "wss://" + host + ":" + port + "/p"
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

	// handler to ask local proxy
	proxy := createProxy()
	proxyHandler := LogHandler("proxy  <--", proxy)

	// handler to ask remote proxy
	remoteProxy := LogHandler("remote <--", createRemoteProxy(proxy, pUrl, "", origin))

	// cache handler
	hmap := map[string]http.Handler{
		"proxy":  proxyHandler,
		"remote": remoteProxy,
		"block": LogHandler("block  <--", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := http.StatusForbidden
			http.Error(w, http.StatusText(s), s)
		})),
	}
	f, err := os.OpenFile("data.txt", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	cache := NewCacheHandler(nil, hmap, f)
	cache.SaveW = f

	// default handler
	defProxy := createProxy()
	defProxy.Fallback = remoteProxy
	defProxy.ValidHTTP = func(req *http.Request, resp *http.Response) error {
		err := validHTTP(req, resp)
		if err == nil {
			//go cache.Set(req.Host, "", proxyHandler)
		} else {
			go cache.Set(req.Host, "remote", remoteProxy)
		}
		return err
	}
	defProxy.ValidConnect = func(req *http.Request, c net.Conn) error {
		err := handshakeConnect(req.URL, c)
		if err == nil {
			go cache.Set(req.Host, "", proxyHandler)
		} else {
			go cache.Set(req.Host, "remote", remoteProxy)
		}
		return err
	}
	cache.Default = LogHandler("       <--", defProxy)

	fmt.Println("Start listening", ":"+localPort)
	err = http.ListenAndServe(":"+localPort, cache)

	//err = http.ListenAndServe(":"+localPort, LogHandler(proxy))
	if err != nil {
		panic(err)
	}
}

func createFallback(name string, h http.Handler, cache *CacheHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go cache.Set(r.Host, name, h)
		h.ServeHTTP(w, r)
	})
}

func createProxy() *NTLMProxy {
	proxy, _ := NewNTLMProxy(proxyUrl)
	return proxy
}

var notFound = errors.New("Not found")
var filtered = errors.New("Filtered")

func validHTTP(req *http.Request, resp *http.Response) error {
	switch {
	case resp.StatusCode >= 400:
		return notFound
	case resp.StatusCode >= 300 && resp.StatusCode != http.StatusNotModified:
		l := resp.Header.Get("Location")
		u, err := url.Parse(l)
		if err != nil {
			return err
		}
		if u.Host == "alert.scansafe.net" {
			return filtered
		}
	}
	return nil
}

func hasPort(s string) bool { return strings.LastIndex(s, ":") > strings.LastIndex(s, "]") }

func handshakeConnect(u *url.URL, c net.Conn) error {
	h := u.Host
	if hasPort(h) {
		h = h[:strings.LastIndex(h, ":")]
	}
	cfg := &tls.Config{ServerName: h}
	tlsConn := tls.Client(c, cfg)
	return tlsConn.Handshake()
}

func createRemoteProxy(proxy *NTLMProxy, pUrl, protocol, origin string) http.Handler {
	genConn := func() (net.Conn, error) {
		//conn, err := ProxyDial(pUrl, "", origin)
		conn, err := proxy.Websocket(pUrl, "", origin)
		if err != nil {
			return nil, err
		}
		conn.PayloadType = websocket.BinaryFrame
		return fetch.NewClientConn(conn, 0x56), nil
	}
	genConn = logConnect(genConn)
	return Tunnel(NewConnPool(genConn))
}
