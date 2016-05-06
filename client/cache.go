package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/cznic/b"
)

type item struct {
	name string
	h    http.Handler
}

type SyncWriter interface {
	io.Writer
	Sync() error
	Seek(offset int64, whence int) (ret int64, err error)
}

type CacheHandler struct {
	tree    *b.Tree
	Default http.Handler
	lock    sync.RWMutex

	SaveW   SyncWriter
	writing int32
}

var ErrWriting = errors.New("cache is writing")

func compareHost(a, b string) int {
	var i int
	an, bn := len(a), len(b)
	var as, bs byte
	for i = 1; i <= an && i <= bn; i++ {
		as, bs = a[an-i], b[bn-i]
		if as != bs {
			switch {
			case i == an && as == '*', i == bn && bs == '*':
				return 0
			case as > bs:
				return +1
			default:
				return -1
			}
		}
	}
	return 0
}

func NewCacheHandler(h http.Handler, hmap map[string]http.Handler, r io.Reader) *CacheHandler {
	c := &CacheHandler{
		tree: b.TreeNew(func(a, b interface{}) int {
			as, bs := a.(string), b.(string)
			return compareHost(as, bs)
		}),
		Default: h,
	}
	if r != nil {
		c.Read(r, hmap)
	}
	return c
}

func (c *CacheHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.lock.RLock()
	e, ok := c.tree.Seek(r.Host)
	c.lock.RUnlock()

	if ok {
		_, v, _ := e.Next()
		v.(*item).h.ServeHTTP(w, r)
		e.Close()
		return
	}
	if c.Default == nil {
		http.Error(w, "No handler", http.StatusInternalServerError)
		return
	}
	c.Default.ServeHTTP(w, r)
}

func (c *CacheHandler) Set(addr, name string, h http.Handler) {
	c.lock.Lock()
	c.set(addr, name, h)
	c.lock.Unlock()

	if name != "" && c.SaveW != nil {
		go func() {
			if !atomic.CompareAndSwapInt32(&c.writing, 0, 1) {
				return
			}
			defer atomic.StoreInt32(&c.writing, 0)
			c.SaveW.Seek(0, 0)
			c.Save(c.SaveW)
			c.SaveW.Sync()
		}()
	}
}

func (c *CacheHandler) set(addr, name string, h http.Handler) {
	c.tree.Set(addr, &item{name: name, h: h})
}

func (c *CacheHandler) Save(w io.Writer) error {
	c.lock.RLock()
	defer c.lock.RUnlock()

	e, err := c.tree.SeekFirst()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	defer e.Close()

	for err == nil {
		var k, v interface{}
		k, v, err = e.Next()
		if err != nil {
			break
		}
		name := v.(*item).name
		if name != "" {
			fmt.Fprintf(w, "%s\t%s\n", k.(string), name)
		}
	}

	if err == io.EOF {
		return nil
	}
	return err
}

func (c *CacheHandler) Read(r io.Reader, hmap map[string]http.Handler) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	scr := bufio.NewScanner(r)
	for scr.Scan() {
		t := strings.Split(scr.Text(), "\t")
		if len(t) != 2 {
			log.Print("Failed to parse line: " + scr.Text())
			continue
		}
		if h, ok := hmap[t[1]]; ok {
			c.set(t[0], t[1], h)
		} else {
			log.Print("Failed to find the handler: " + t[0])
		}
	}
	return scr.Err()
}
