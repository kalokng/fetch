package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/websocket"

	"github.com/alexbrainman/sspi"
	"github.com/alexbrainman/sspi/ntlm"
)

var encoder = base64.StdEncoding

type NTLMProxy struct {
	cred      *sspi.Credentials
	transport *http.Transport
	proxyUrl  *url.URL

	ValidHTTP    func(req *http.Request, resp *http.Response) error
	ValidConnect func(req *http.Request, c net.Conn) error
	Fallback     http.Handler
}

type bytePool sync.Pool

var ntlmPool = bytePool(sync.Pool{
	New: func() interface{} {
		return make([]byte, ntlm.PackageInfo.MaxToken)
	},
})

func (p *bytePool) Get() []byte {
	return (*sync.Pool)(p).Get().([]byte)
}

func (p *bytePool) Put(b []byte) {
	(*sync.Pool)(p).Put(b)
}

func NewNTLMProxy(proxyUrl string) (*NTLMProxy, error) {
	pUrl, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}
	cred, err := ntlm.AcquireCurrentUserCredentials()
	if err != nil {
		return nil, err
	}
	p := &NTLMProxy{
		cred: cred,
		transport: &http.Transport{
			Proxy: func(r *http.Request) (*url.URL, error) {
				if r.URL.Scheme == "https" {
					return nil, nil
				}
				return pUrl, nil
			},
			Dial: net.Dial,
		},
		proxyUrl: pUrl,
	}
	return p, nil
}

/*
func (p *NTLMProxy) dial(method, address string) (net.Conn, error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return err
	}
}
*/

func (p *NTLMProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "CONNECT":
		p.handleConnect(w, r)
	default:
		p.handleHTTP(w, r)
	}
}

func (p *NTLMProxy) dial(r *url.URL) (net.Conn, error) {
	context, b, err := ntlm.NewClientContext(p.cred)
	if err != nil {
		return nil, errors.New("Cannot create client context: " + err.Error())
	}

	remote, err := p.transport.Dial("tcp", p.proxyUrl.Host)
	if err != nil {
		return nil, errors.New("Failed to dial: " + err.Error())
	}

	pr := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{},
		Host:   r.Host,
		Header: make(http.Header),
	}

	cb := ntlmPool.Get()
	defer func() {
		ntlmPool.Put(cb)
	}()

	// 1st request: Client -> Proxy handshake
	n := encoder.EncodedLen(len(b))
	if cap(cb) < n {
		cb = make([]byte, n)
	}
	encoder.Encode(cb, b)
	pr.Header.Set("Proxy-Authorization", fmt.Sprintf("NTLM %s", cb[:n]))
	pr.Write(remote)

	// Read response.
	// Okay to use and discard buffered reader here, because
	// TLS server will not speak until spoken to.
	br := bufio.NewReader(remote)
	resp, err := http.ReadResponse(br, pr)
	if err != nil {
		remote.Close()
		return nil, errors.New("server failed response: " + err.Error())
	}
	//dumpResp(resp, true)

	// 1st reply: Proxy -> Client challenge
	// If the proxy didn't request for challenge, just tunnel the connection if ok
	if resp.StatusCode != http.StatusProxyAuthRequired {
		if resp.StatusCode != http.StatusOK {
			// we do not know how to handle it, just let user do it
			remote.Close()
			return nil, errors.New("Unknown challenge: " + resp.Status)
		}

		return remote, nil
	}

	// comsume the body
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	auth := resp.Header.Get("Proxy-Authenticate")
	f := strings.SplitN(auth, " ", 2)
	if len(f) < 2 {
		remote.Close()
		return nil, errors.New("Unknown Proxy-Authenticate: " + auth)
	}
	encodedChg := f[1]
	challenge := ntlmPool.Get()
	defer func() {
		ntlmPool.Put(challenge)
	}()
	n = encoder.EncodedLen(len(encodedChg))
	if cap(challenge) < n {
		challenge = make([]byte, n)
	}
	_, err = encoder.Decode(challenge, []byte(encodedChg))
	if err != nil {
		remote.Close()
		return nil, errors.New("Cannot decode challenge: " + auth)
	}

	b, err = context.Update(challenge[:n])
	if err != nil {
		remote.Close()
		return nil, errors.New("Failed to response challenge: " + err.Error())
	}

	// 2nd request: Client -> Proxy response
	n = encoder.EncodedLen(len(b))
	if cap(cb) < n {
		cb = make([]byte, n)
	}
	encoder.Encode(cb, b)
	pr.Header.Set("Proxy-Connection", "Keep-Alive")
	pr.Header.Set("Proxy-Authorization", fmt.Sprintf("NTLM %s", cb[:n]))

	pr.Write(remote)
	resp, err = http.ReadResponse(br, pr)
	if err != nil {
		remote.Close()
		return nil, errors.New("Failed reply challenge: " + err.Error())
	}

	// something goes wrong... let the user handle it
	if resp.StatusCode != http.StatusOK {
		remote.Close()
		return nil, errors.New("Unknown error: " + resp.Status)
	}

	// reply to client
	return remote, nil
}

func (p *NTLMProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	remote, err := p.dial(r.URL)
	if err != nil {
		http.Error(w, "Failed to establish tunnel connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if p.ValidConnect != nil {
		err := p.ValidConnect(r, remote)
		if err != nil {
			remote.Close()
			log.Print("Failed to establish connection: " + err.Error())
			if p.Fallback == nil {
				http.Error(w, "Failed to establish connection: "+err.Error(), http.StatusInternalServerError)
			} else {
				p.Fallback.ServeHTTP(w, r)
			}
			return
		}
		remote, err = p.dial(r.URL)
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		fmt.Println(err.Error())
		remote.Close()
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		fmt.Println(err.Error())
		remote.Close()
		return
	}

	// comsume whatever the server says
	//io.Copy(ioutil.Discard, resp.Body)
	//resp.Body.Close()
	conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))

	go copyAndClose(remote, conn)
	go copyAndClose(conn, remote)
}

func (p *NTLMProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	// temporary store the body, so will not be consumed in handshake
	body := r.Body
	method := r.Method
	length := r.ContentLength
	r.Body = nil
	r.Method = "GET"
	r.ContentLength = 0

	// Create client context for NTLM challenge
	context, b, err := ntlm.NewClientContext(p.cred)
	if err != nil {
		http.Error(w, "Cannot create client context: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cb := ntlmPool.Get()
	defer func() {
		ntlmPool.Put(cb)
	}()

	// 1st request: Client -> Proxy handshake
	n := encoder.EncodedLen(len(b))
	if cap(cb) < n {
		cb = make([]byte, n)
	}
	encoder.Encode(cb, b)
	r.Header.Set("Proxy-Authorization", fmt.Sprintf("NTLM %s", cb[:n]))

	resp, err := p.transport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 1st reply: Proxy -> Client challenge
	// If the proxy didn't request for challenge, just send back to client
	if resp.StatusCode != http.StatusProxyAuthRequired {
		if body != nil || method != "GET" {
			io.Copy(ioutil.Discard, resp.Body)
			// Resend the request again, with body and correct method
			r.Body = body
			r.Method = method
			r.ContentLength = length
			resp, err = p.transport.RoundTrip(r)
			if err != nil {
				http.Error(w, "Failed to get response: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		pushResponse(w, resp)
		return
	}

	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	auth := resp.Header.Get("Proxy-Authenticate")
	f := strings.SplitN(auth, " ", 2)
	if len(f) < 2 {
		http.Error(w, "Unknown Proxy-Authenticate: "+auth, http.StatusInternalServerError)
		return
	}
	encodedChg := f[1]
	challenge := ntlmPool.Get()
	defer func() {
		ntlmPool.Put(challenge)
	}()
	n = encoder.EncodedLen(len(encodedChg))
	if cap(challenge) < n {
		challenge = make([]byte, n)
	}
	_, err = encoder.Decode(challenge, []byte(encodedChg))
	if err != nil {
		http.Error(w, "Cannot decode challenge: "+auth, http.StatusInternalServerError)
		return
	}

	b, err = context.Update(challenge[:n])
	if err != nil {
		http.Error(w, "Failed to response challenge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 2nd request: Client -> Proxy response
	n = encoder.EncodedLen(len(b))
	if cap(cb) < n {
		cb = make([]byte, n)
	}
	encoder.Encode(cb, b)
	r.Header.Set("Proxy-Connection", "Keep-Alive")
	r.Header.Set("Proxy-Authorization", fmt.Sprintf("NTLM %s", cb[:n]))

	r.Body = body
	r.Method = method
	r.ContentLength = length
	resp, err = p.transport.RoundTrip(r)
	if err != nil {
		http.Error(w, "Failed to get response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	//dumpResp(resp, true)

	// if response is not valid, fallback
	if p.ValidHTTP != nil {
		if err := p.ValidHTTP(r, resp); err != nil {
			if p.Fallback != nil {
				p.Fallback.ServeHTTP(w, r)
				return
			}
			log.Print("Invalid response from " + r.Host)
		}
	}

	pushResponse(w, resp)
}

func (p *NTLMProxy) Websocket(url_, protocol, origin string) (ws *websocket.Conn, err error) {
	config, err := websocket.NewConfig(url_, origin)
	if err != nil {
		return nil, err
	}

	if protocol != "" {
		config.Protocol = []string{protocol}
	}

	rpUrl, err := url.Parse(url_)
	if err != nil {
		return nil, err
	}

	client, err := p.dial(rpUrl)
	if err != nil {
		return nil, err
	}

	return websocket.NewClient(config, client)
}

func pushResponse(w http.ResponseWriter, resp *http.Response) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	resp.Write(conn)
}

func dumpReq(req *http.Request, body bool) {
	b, _ := httputil.DumpRequest(req, body)
	fmt.Printf("%s\n", b)
}

func dumpResp(resp *http.Response, body bool) {
	b, _ := httputil.DumpResponse(resp, body)
	fmt.Printf("%s\n", b)
}

func copyAndClose(to, fr net.Conn) {
	io.Copy(to, fr)
	to.Close()
}
