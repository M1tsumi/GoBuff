package gobuff

import (
	"io"
)

// Buffer is a reusable byte buffer with explicit growth strategy.
// It keeps a read cursor (r) so repeated Read calls work as expected.
type Buffer struct {
	buf []byte
	r   int
}

// NewBuffer creates a buffer with an optional initial capacity.
func NewBuffer(initialCap int) *Buffer {
	if initialCap < 0 {
		initialCap = 0
	}
	return &Buffer{buf: make([]byte, 0, initialCap)}
}

// Bytes returns the unread contents of the buffer.
func (b *Buffer) Bytes() []byte {
	return b.buf[b.r:]
}

// UnsafeBytes exposes the full underlying slice (including consumed bytes).
// Use only when you need zero-copy access; mutations affect the buffer.
func (b *Buffer) UnsafeBytes() []byte {
	return b.buf
}

// String returns the unread contents of the buffer as a string.
func (b *Buffer) String() string {
	return string(b.Bytes())
}

// Len returns the number of unread bytes.
func (b *Buffer) Len() int {
	return len(b.buf) - b.r
}

// Cap returns the total buffer capacity.
func (b *Buffer) Cap() int {
	return cap(b.buf)
}

// Grow ensures the buffer can accommodate n additional bytes.
func (b *Buffer) Grow(n int) {
	if n > 0 {
		b.grow(n)
	}
}

// Reserve grows the buffer and returns a slice of length n backed by the buffer
// for zero-copy writes. The caller must not let the returned slice escape
// beyond the buffer's lifetime without Put-ing the buffer back to a pool.
func (b *Buffer) Reserve(n int) []byte {
	if n <= 0 {
		return nil
	}
	b.grow(n)
	prev := len(b.buf)
	b.buf = b.buf[:prev+n]
	return b.buf[prev:]
}

// Reset clears the buffer to empty.
func (b *Buffer) Reset() {
	b.buf = b.buf[:0]
	b.r = 0
}

// Write appends p to the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.r >= len(b.buf) {
		b.Reset()
	}
	b.grow(len(p))
	b.buf = append(b.buf, p...)
	return len(p), nil
}

// WriteByte appends a single byte.
func (b *Buffer) WriteByte(v byte) error {
	if b.r >= len(b.buf) {
		b.Reset()
	}
	b.grow(1)
	b.buf = append(b.buf, v)
	return nil
}

// WriteString appends a string to the buffer.
func (b *Buffer) WriteString(s string) (int, error) {
	if len(s) == 0 {
		return 0, nil
	}
	if b.r >= len(b.buf) {
		b.Reset()
	}
	b.grow(len(s))
	b.buf = append(b.buf, s...)
	return len(s), nil
}

// Read copies data from the buffer into p.
// It returns io.EOF when no data remains.
func (b *Buffer) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if b.r >= len(b.buf) {
		b.Reset()
		return 0, io.EOF
	}
	n := copy(p, b.buf[b.r:])
	b.r += n
	if b.r >= len(b.buf) {
		b.Reset()
	}
	return n, nil
}

// WriteTo implements io.WriterTo.
func (b *Buffer) WriteTo(w io.Writer) (int64, error) {
	if b.r >= len(b.buf) {
		b.Reset()
		return 0, nil
	}
	p := b.Bytes()
	n, err := w.Write(p)
	if n > 0 {
		b.r += n
		if b.r >= len(b.buf) {
			b.Reset()
		}
	}
	if err == nil && n != len(p) {
		return int64(n), io.ErrShortWrite
	}
	return int64(n), err
}

// ReadFrom implements io.ReaderFrom.
func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	var total int64
	if b.r >= len(b.buf) {
		b.Reset()
	}
	const minRead = 512
	for {
		// Ensure there is space to read into.
		if len(b.buf) == cap(b.buf) {
			b.grow(minRead)
		}
		start := len(b.buf)
		b.buf = b.buf[:cap(b.buf)]
		n, err := r.Read(b.buf[start:])
		if n > 0 {
			b.buf = b.buf[:start+n]
			total += int64(n)
		} else {
			b.buf = b.buf[:start]
		}
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

// grow ensures capacity for n additional bytes using power-of-two growth.
func (b *Buffer) grow(n int) {
	if n <= 0 {
		return
	}
	if b.r >= len(b.buf) {
		b.Reset()
	}
	// Fast path: enough free capacity at the end.
	if cap(b.buf)-len(b.buf) >= n {
		return
	}
	// Reclaim space from consumed bytes by compacting.
	if b.r > 0 {
		unread := len(b.buf) - b.r
		if unread+n <= cap(b.buf) {
			copy(b.buf[:unread], b.buf[b.r:])
			b.buf = b.buf[:unread]
			b.r = 0
			return
		}
	}
	// Allocate a new slice sized for unread data + n.
	unread := len(b.buf) - b.r
	required := unread + n
	newCap := nextPowerOfTwo(required)
	newBuf := make([]byte, unread, newCap)
	copy(newBuf, b.buf[b.r:])
	b.buf = newBuf
	b.r = 0
}

func nextPowerOfTwo(n int) int {
	if n <= 0 {
		return 0
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	if n < 0 {
		return 0
	}
	return n + 1
}
