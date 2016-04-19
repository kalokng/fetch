package main

import (
	"errors"
	"net"
	"sync"
)

var NoConnection = errors.New("No connection can be established")

type ConnPool struct {
	pool sync.Pool
}

func (p *ConnPool) Get() (net.Conn, error) {
	v := p.pool.Get()
	conn, ok := v.(net.Conn)
	if conn == nil || !ok {
		if err, ok := v.(error); ok {
			return nil, err
		}
		return nil, NoConnection
	}
	return conn, nil
}

func (p *ConnPool) Put(conn net.Conn) {
	p.pool.Put(conn)
}

func NewConnPool(newConn func() (net.Conn, error)) *ConnPool {
	return &ConnPool{
		pool: sync.Pool{
			New: func() interface{} {
				conn, err := newConn()
				if err != nil {
					return err
				}
				return conn
			},
		},
	}
}
