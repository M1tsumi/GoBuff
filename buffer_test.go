package gobuff

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestBufferReserveAndLen(t *testing.T) {
	b := NewBuffer(0)
	slot := b.Reserve(5)
	copy(slot, []byte("hello"))
	if b.Len() != 5 {
		t.Fatalf("expected len 5, got %d", b.Len())
	}
	if got := b.String(); got != "hello" {
		t.Fatalf("unexpected contents: %q", got)
	}
}

func TestBufferWriteToReadFrom(t *testing.T) {
	src := NewBuffer(0)
	_, _ = src.WriteString("data")

	var dst bytes.Buffer
	n, err := src.WriteTo(&dst)
	if err != nil || n != 4 {
		t.Fatalf("WriteTo n=%d err=%v", n, err)
	}
	if dst.String() != "data" {
		t.Fatalf("dst mismatch: %q", dst.String())
	}

	recv := NewBuffer(0)
	n2, err := recv.ReadFrom(strings.NewReader("more"))
	if err != nil || n2 != 4 {
		t.Fatalf("ReadFrom n=%d err=%v", n2, err)
	}
	if recv.String() != "more" {
		t.Fatalf("recv mismatch: %q", recv.String())
	}
}

func TestBufferUnsafeBytes(t *testing.T) {
	b := NewBuffer(4)
	_, _ = b.Write([]byte{1, 2})
	raw := b.UnsafeBytes()
	if len(raw) != 2 {
		t.Fatalf("expected raw len 2, got %d", len(raw))
	}
	raw[0] = 9
	if b.Bytes()[0] != 9 {
		t.Fatalf("expected mutation visible in buffer")
	}
}

func TestBufferReadEOF(t *testing.T) {
	b := NewBuffer(0)
	buf := make([]byte, 1)
	if _, err := b.Read(buf); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}
