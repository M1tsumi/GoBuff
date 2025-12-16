//go:build bytebufferpool
// +build bytebufferpool

package gobuff

import (
	"testing"

	"github.com/valyala/bytebufferpool"
)

func BenchmarkByteBufferPool(b *testing.B) {
	payload := []byte("hello world")
	pool := bytebufferpool.Pool{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		buf.Reset()
		_, _ = buf.Write(payload)
		_ = buf.Bytes()
		pool.Put(buf)
	}
}
