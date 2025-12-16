//go:build bufpool
// +build bufpool

package gobuff

import (
	"testing"

	"github.com/vmihailenco/bufpool"
)

func BenchmarkVmihailencoBufpool(b *testing.B) {
	payload := []byte("hello world")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bufpool.Get(64)
		buf.Reset()
		_, _ = buf.Write(payload)
		sinkBytes = buf.Bytes()
		bufpool.Put(buf)
	}
}
