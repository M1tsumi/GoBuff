//go:build bpool
// +build bpool

package gobuff

import (
	"bytes"
	"testing"

	"github.com/oxtoacart/bpool"
)

func BenchmarkBPoolBufferPool(b *testing.B) {
	payload := []byte("hello world")
	bpoolInst := bpool.NewBufferPool(64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bpoolInst.Get()
		buf.Reset()
		buf.Write(payload)
		sinkBytes = buf.Bytes()
		bpoolInst.Put(buf)
	}
}

func BenchmarkBPoolBytesBufferReuse(b *testing.B) {
	payload := []byte("hello world")
	var buf bytes.Buffer
	buf.Grow(64)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.Write(payload)
		sinkBytes = buf.Bytes()
	}
}
