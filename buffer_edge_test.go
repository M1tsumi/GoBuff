package gobuff

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

type shortWriter struct {
	w     io.Writer
	limit int
}

func (s shortWriter) Write(p []byte) (int, error) {
	if len(p) > s.limit {
		p = p[:s.limit]
	}
	return s.w.Write(p)
}

func TestBufferCompactionPreservesData(t *testing.T) {
	b := NewBuffer(8)
	_, _ = b.WriteString("abcdefgh")

	tmp := make([]byte, 3)
	if n, err := b.Read(tmp); err != nil || n != 3 {
		t.Fatalf("read n=%d err=%v", n, err)
	}
	if string(tmp) != "abc" {
		t.Fatalf("unexpected read: %q", string(tmp))
	}

	capBefore := b.Cap()
	_, _ = b.WriteString("XYZ")
	if b.Cap() != capBefore {
		t.Fatalf("expected compaction/reuse without growth; cap before=%d after=%d", capBefore, b.Cap())
	}
	if got := b.String(); got != "defghXYZ" {
		t.Fatalf("unexpected contents: %q", got)
	}
}

func TestBufferWriteToShortWrite(t *testing.T) {
	b := NewBuffer(0)
	_, _ = b.WriteString("hello")

	var dst bytes.Buffer
	n, err := b.WriteTo(shortWriter{w: &dst, limit: 2})
	if err != io.ErrShortWrite {
		t.Fatalf("expected ErrShortWrite, got %v", err)
	}
	if n != 2 {
		t.Fatalf("expected n=2, got %d", n)
	}
	if dst.String() != "he" {
		t.Fatalf("dst mismatch: %q", dst.String())
	}
	if got := b.String(); got != "llo" {
		t.Fatalf("buffer mismatch after short write: %q", got)
	}
}

func TestBufferReadFromExact(t *testing.T) {
	b := NewBuffer(0)
	n, err := b.ReadFrom(strings.NewReader("more"))
	if err != nil || n != 4 {
		t.Fatalf("ReadFrom n=%d err=%v", n, err)
	}
	if b.String() != "more" {
		t.Fatalf("unexpected contents: %q", b.String())
	}
}
