//go:build libp2pbufferpool
// +build libp2pbufferpool

package gobuff

import (
	"testing"

	pool "github.com/libp2p/go-buffer-pool"
)

func BenchmarkLibp2pGlobalPoolBytes(b *testing.B) {
	payload := []byte("hello world")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get(64)
		buf = buf[:0]
		buf = append(buf, payload...)
		sinkBytes = buf
		pool.Put(buf)
	}
}
