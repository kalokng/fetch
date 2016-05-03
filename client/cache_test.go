package main

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestCache(t *testing.T) {
	out := make(chan string, 1)
	h := make(map[string]http.Handler, 5)
	for _, v := range strings.Split("12345", "") {
		v := v
		h[v] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			out <- v
		})
	}
	d := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		out <- "def"
	})

	c := NewCacheHandler(d, h, nil)
	f, err := os.OpenFile("data.txt", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	c.SaveW = f
	for k, v := range h {
		c.Set(k+k, k, v)
	}
	req := &http.Request{}
	for _, v := range strings.Split("00;11;22;33;44;55;66;0;1;2;3;4;5;6", ";") {
		req.Host = v
		c.ServeHTTP(nil, req)
		select {
		case s := <-out:
			if v[:1] != s {
				switch s {
				case "def":
				default:
					t.Fatalf("%s : %s", v, s)
				}
			}
		default:
			t.Fatal("No out")
		}
	}
	var obuf bytes.Buffer
	if err := c.Save(&obuf); err != nil {
		t.Fatal(err)
	}

	nc := NewCacheHandler(d, h, &obuf)
	for _, v := range strings.Split("00;11;22;33;44;55;66;0;1;2;3;4;5;6", ";") {
		req.Host = v
		nc.ServeHTTP(nil, req)
		select {
		case s := <-out:
			if v[:1] != s {
				switch s {
				case "def":
				default:
					t.Fatalf("%s : %s", v, s)
				}
			}
		default:
			t.Fatal("No out")
		}
	}
}
