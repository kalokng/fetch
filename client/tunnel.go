package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func Tunnel(host string, pool *ConnPool) error {
	fmt.Println("Start listening", host)
	return http.ListenAndServe(host, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := pool.Get()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Println("Received", r.URL.Path)
		//fmt.Fprintln(conn, "Received", r.URL.Path)
		hj, ok := w.(http.Hijacker)
		if !ok {
			panic("CANNOT hijack")
		}
		c, bufrw, err := hj.Hijack()
		if err != nil {
			panic(err)
		}

		mw := io.MultiWriter(conn, os.Stdout)
		// send out the request
		r.Write(mw)

		go func() {
			io.Copy(mw, bufrw)
			pool.Put(conn)
		}()
		io.Copy(bufrw, conn)
		c.Close()
	}))
}
