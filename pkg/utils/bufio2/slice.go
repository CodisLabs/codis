package bufio2

type sliceAlloc struct {
	buf []byte
	off int
}

func (d *sliceAlloc) Make(n int) []byte {
	if n >= 512 {
		return make([]byte, n)
	}
	if max := len(d.buf) - d.off; max < n {
		d.buf = make([]byte, 8192)
		d.off = 0
	}
	n += d.off
	s := d.buf[d.off:n:n]
	d.off = n
	return s
}
