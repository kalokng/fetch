package fetch

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type utf8Conn struct {
	net.Conn
	w io.Writer
	r io.Reader
}

// NewUtf8Conn wraps c so it can send any bytes in utf-8 compatibility
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

type serverConn struct {
	net.Conn
	w io.Writer
}

// NewServerConn wraps c so it will XOR the sending streams with mask byte.
func NewServerConn(c net.Conn, mask byte) net.Conn {
	w := NewMaskWriter(c, mask)
	return &serverConn{
		Conn: c,
		w:    w,
	}
}

func (c *serverConn) Write(p []byte) (int, error) {
	return c.w.Write(p)
}

type WsConnWrapper struct {
	*websocket.Conn
	r  io.Reader
	wc io.WriteCloser
}

func (ws *WsConnWrapper) Read(p []byte) (int, error) {
	if ws.r == nil {
		var err error
		_, ws.r, err = ws.Conn.NextReader()
		if err != nil {
			return 0, err
		}
	}
	n, err := ws.r.Read(p)
	if err == io.EOF {
		ws.r = nil
		err = nil
	}
	return n, err
}

func (ws *WsConnWrapper) Write(p []byte) (int, error) {
	if ws.wc == nil {
		var err error
		ws.wc, err = ws.Conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return 0, err
		}
	}
	n, err := ws.wc.Write(p)
	return n, err
}

func (ws *WsConnWrapper) Close() error {
	if ws.wc == nil {
		return nil
	}
	err := ws.wc.Close()
	ws.wc = nil
	return err
}

func (ws *WsConnWrapper) SetDeadline(t time.Time) error {
	re := ws.SetReadDeadline(t)
	we := ws.SetWriteDeadline(t)
	if re != nil {
		return re
	}
	return we
}

type clientConn struct {
	net.Conn
	r io.Reader
}

// NewClientConn wraps c so it will XOR the receiving streams with mask byte
func NewClientConn(c net.Conn, mask byte) net.Conn {
	r := NewMaskReader(c, mask)
	return &clientConn{
		Conn: c,
		r:    r,
	}
}

func (c *clientConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
