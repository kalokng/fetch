package main

import (
	"io"
	"log"
	"net/http"
)

func LogHandler(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		go log.Printf("%s %s: %s", prefix, r.Method, r.URL.Host)
		h.ServeHTTP(w, r)
	})
}

func Tunnel(pool *ConnPool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := pool.Get()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//{
		//	if b, err := httputil.DumpRequest(r, false); err == nil {
		//		fmt.Printf("%s\n", b)
		//	}
		//}

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
		if r.Method == "CONNECT" {
			go func() {
				r.Write(conn)
				//io.Copy(conn, io.TeeReader(c, os.Stdout))
				io.Copy(conn, c)
				pool.Put(conn)
			}()
			//io.Copy(io.MultiWriter(c, os.Stdout), conn)
			io.Copy(c, conn)
			c.Close()
			return
		}

		go r.Write(conn)

		io.Copy(c, conn)
		//io.Copy(conn, c)
		c.Close()
		pool.Put(conn)
	})
}
