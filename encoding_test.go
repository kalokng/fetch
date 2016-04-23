package fetch

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"testing"
)

type dataType int

const (
	genAcs = iota
	genDecs
	genRand
)

type genData func(n int, min, max byte) []byte

func dataAcs(n int, min, max byte) []byte {
	b := make([]byte, n)
	x := min
	for i := range b {
		b[i] = x
		if x+1 >= max {
			x = min
		} else {
			x++
		}
	}
	return b
}

func dataDecs(n int, min, max byte) []byte {
	b := make([]byte, n)
	x := max
	for i := range b {
		b[i] = x
		if x-1 <= min {
			x = max
		} else {
			x--
		}
	}
	return b
}

func dataRand(n int, min, max byte) []byte {
	b := make([]byte, n)
	rand.Read(b)
	r := int(max - min + 1)
	for i, v := range b {
		if v < min || v > max {
			b[i] = byte(rand.Intn(r)) + min
		}
	}
	return b
}

func compare(t *testing.T, a, b []byte) {
	if len(a) != len(b) {
		t.Errorf("Wrong count: %d %d", len(a), len(b))
		return
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("Wrong value: at %d: %x %x", i, a[i], b[i])
			return
		}
	}
}

func method(t dataType) (name string, f genData) {
	switch t {
	case genAcs:
		return "dataAcs", dataAcs
	case genDecs:
		return "dataDecs", dataDecs
	case genRand:
		return "dataRand", dataRand
	default:
		return "unknown", dataAcs
	}
}

func TestEncode(t *testing.T) {
	for _, dt := range []dataType{genAcs, genDecs, genRand} {
		for _, size := range []int{0, 1, 2, 10, 256, 1023, 1024, 1025, 2048, 9999} {
			name, method := method(dt)
			fmt.Println(name, size)
			b := method(size, 0, 255)

			r1, w1 := io.Pipe()
			r2, w2 := io.Pipe()
			var buf bytes.Buffer
			wr, ww := NewDecoder(r2), NewEncoder(w1)
			go func() {
				ww.Write(b)
				w1.Close()
			}()
			go func() {
				buf.ReadFrom(wr)
			}()
			io.Copy(w2, r1)

			compare(t, b, buf.Bytes())
		}
	}
}
