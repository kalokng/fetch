package fetch

import (
	"io"
	"sync"
)

type bytePool sync.Pool

var pool = bytePool(sync.Pool{
	New: func() interface{} {
		return make([]byte, 2048)
	},
})

func (p *bytePool) Get() []byte {
	return (*sync.Pool)(p).Get().([]byte)
}

func (p *bytePool) Put(b []byte) {
	(*sync.Pool)(p).Put(b)
}

type MaskWriter struct {
	io.Writer
	Mask byte
}

func NewMaskWriter(w io.Writer, mask byte) *MaskWriter {
	return &MaskWriter{Writer: w, Mask: mask}
}

func (w MaskWriter) Write(p []byte) (n int, err error) {
	b := pool.Get()
	defer pool.Put(b)

	var nn int
	for nn < len(p) {
		bn := copy(b, p[nn:])
		nn += bn
		for i := range b[:bn] {
			b[i] ^= w.Mask
		}
		wn, err := w.Writer.Write(b[:bn])
		n += wn
		if err != nil {
			return n, err
		}
	}
	return
}

type MaskReader struct {
	io.Reader
	Mask byte
}

func NewMaskReader(r io.Reader, mask byte) *MaskReader {
	return &MaskReader{Reader: r, Mask: mask}
}

func (r MaskReader) Read(p []byte) (n int, err error) {
	b := pool.Get()
	defer pool.Put(b)

	for n < len(p) {
		var l int
		if len(b) > len(p)-n {
			l = len(p) - n
		} else {
			l = len(b)
		}
		bn, err := r.Reader.Read(b[:l])
		if err != nil {
			return n, err
		}

		for i := range b[:bn] {
			b[i] ^= r.Mask
		}
		rn := copy(p[n:], b[:bn])
		n += rn
		if rn < l {
			break
		}
	}
	return
}
