package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/kalokng/fetch"
)

func Tunnel(pool *ConnPool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := pool.Get()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Println("Received", r.Host, r.URL.Path)
		//fmt.Fprintln(conn, "Received", r.URL.Path)
		hj, ok := w.(http.Hijacker)
		if !ok {
			panic("CANNOT hijack")
		}
		c, _, err := hj.Hijack()
		if err != nil {
			panic(err)
		}

		//mw := io.MultiWriter(conn, os.Stdout)
		// send out the request
		fmt.Println(r.Method, r.URL.String())
		if r.Method == "CONNECT" {
			go func() {
				ew := fetch.NewEncoder(conn)
				r.Write(ew)
				//io.Copy(ew, io.TeeReader(c, os.Stdout))
				io.Copy(ew, c)
				//ew.Close()
			}()
			er := fetch.NewDecoder(conn)
			//io.Copy(io.MultiWriter(c, os.Stdout), er)
			io.Copy(c, er)
			c.Close()
			pool.Put(conn)
			return
		}

		go r.Write(conn)

		io.Copy(c, conn)
		//io.Copy(conn, c)
		c.Close()
		pool.Put(conn)
	})
}
