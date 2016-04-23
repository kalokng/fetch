package fetch

import (
	"io"
	"unicode/utf8"
)

const bufsize = 1024

type encoder struct {
	w   io.Writer
	out [bufsize]byte
	r   [utf8.UTFMax]byte
}

func NewEncoder(w io.Writer) io.Writer { return &encoder{w: w} }

func (e *encoder) Write(p []byte) (n int, err error) {
	i := 0
	for _, v := range p {
		if v < utf8.RuneSelf {
			e.out[i] = v
			i++
			if i >= bufsize {
				if _, err := e.w.Write(e.out[:i]); err != nil {
					return n, err
				}
				i = 0
			}
		} else {
			l := utf8.EncodeRune(e.r[:], rune(v))
			if i+l >= bufsize {
				if _, err := e.w.Write(e.out[:i]); err != nil {
					return n, err
				}
				i = 0
			}
			copy(e.out[i:], e.r[:l])
			i += l
		}
		n++
	}
	if i > 0 {
		if _, err := e.w.Write(e.out[:i]); err != nil {
			return n, err
		}
	}
	return
}

type decoder struct {
	r    io.Reader
	buf  [bufsize]byte
	nbuf int
}

func NewDecoder(r io.Reader) io.Reader { return &decoder{r: r} }

func (d *decoder) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		var nn int
		if d.nbuf > 0 {
			j := 0
			for j < d.nbuf && n < len(p) {
				r, size := utf8.DecodeRune(d.buf[j:d.nbuf])
				//fmt.Println(d.nbuf, j, n, r, size)
				if r == utf8.RuneError && size <= 1 {
					// seems buffer missed sth
					break
				}
				if r > 256 {
					panic("r > 256")
				}
				p[n] = byte(r)
				n++
				j += size
			}
			if j > 0 {
				oldn := d.nbuf
				copy(d.buf[:], d.buf[j:d.nbuf])
				d.nbuf -= j
				//fmt.Println("copy", d.nbuf, j)
				if oldn < bufsize || n == len(p) {
					return
				}
			}
		}

		//fmt.Println("err", err)
		if err != nil {
			return n, err
		}

		nn, err = d.r.Read(d.buf[d.nbuf:])
		//fmt.Println(nn, err, d.buf[d.nbuf:nn])
		d.nbuf += nn
	}
	return
}
