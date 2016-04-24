package fetch

import (
	"io"
	"net"
)

type utf8Conn struct {
	net.Conn
	w io.Writer
	r io.Reader
}

func NewUtf8Conn(c net.Conn) net.Conn {
	w := NewEncoder(c)
	r := NewDecoder(c)
	return &utf8Conn{
		Conn: c,
		w:    w,
		r:    r,
	}
}

func (c *utf8Conn) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

func (c *utf8Conn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
