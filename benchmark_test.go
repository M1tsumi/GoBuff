package gobuff

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

var sinkBytes []byte
var sinkInt int

func BenchmarkBufferPoolWriteRead(b *testing.B) {
	pool := NewBufferPoolWithOptions(PoolOptions{
		InitialCap:         64,
		ObserveEvery:       2048,
		SmallLimit:         128,
		CalibrateThreshold: 42000,
		Percentile:         0.95,
	})
	payload := []byte("hello world")
	tmp := make([]byte, len(payload))

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		_, _ = buf.Write(payload)
		n, _ := buf.Read(tmp)
		sinkInt = n
		pool.Put(buf)
	}
}

func BenchmarkBufferWrite(b *testing.B) {
	payload := []byte("hello world")
	tmp := make([]byte, len(payload))
	b.ReportAllocs()
	b.ResetTimer()
	buf := NewBuffer(64)
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_, _ = buf.Write(payload)
		n, _ := buf.Read(tmp)
		sinkInt = n
	}
}

func BenchmarkBufferPoolWriteBytes(b *testing.B) {
	pool := NewBufferPoolWithOptions(PoolOptions{
		InitialCap:         64,
		ObserveEvery:       2048,
		SmallLimit:         128,
		CalibrateThreshold: 42000,
		Percentile:         0.95,
	})
	payload := []byte("hello world")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		_, _ = buf.Write(payload)
		sinkBytes = buf.Bytes()
		pool.Put(buf)
	}
}

func BenchmarkBytesBufferReuse(b *testing.B) {
	payload := []byte("hello world")
	tmp := make([]byte, len(payload))

	b.ReportAllocs()
	b.ResetTimer()

	var buf bytes.Buffer
	buf.Grow(64)
	for i := 0; i < b.N; i++ {
		buf.Reset()
		buf.Write(payload)
		n, _ := buf.Read(tmp)
		sinkInt = n
	}
}

func BenchmarkBufferPoolBuckets(b *testing.B) {
	pool := NewBufferPoolWithOptions(PoolOptions{
		BucketSizes:        []int{64, 256, 1024, 4096},
		ObserveEvery:       2048,
		SmallLimit:         128,
		CalibrateThreshold: 42000,
		Percentile:         0.95,
	})
	payloads := [][]byte{
		make([]byte, 32),
		make([]byte, 200),
		make([]byte, 900),
		make([]byte, 3500),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := payloads[i%len(payloads)]
		buf := pool.GetSized(len(data))
		_, _ = buf.Write(data)
		pool.Put(buf)
	}
}

func BenchmarkBufferWriteTo(b *testing.B) {
	pool := NewBufferPoolWithOptions(PoolOptions{
		InitialCap:         1024,
		ObserveEvery:       2048,
		SmallLimit:         128,
		CalibrateThreshold: 42000,
		Percentile:         0.95,
	})
	payload := bytes.Repeat([]byte("a"), 4096)
	sink := io.Discard
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.GetSized(len(payload))
		_, _ = buf.Write(payload)
		_, _ = buf.WriteTo(sink)
		pool.Put(buf)
	}
}

// Comparison baselines
func BenchmarkSyncPoolBytesBuffer(b *testing.B) {
	var sp sync.Pool
	sp.New = func() any { return &bytes.Buffer{} }
	payload := []byte("hello world")
	tmp := make([]byte, len(payload))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := sp.Get().(*bytes.Buffer)
		buf.Reset()
		buf.Write(payload)
		n, _ := buf.Read(tmp)
		sinkInt = n
		sp.Put(buf)
	}
}

func BenchmarkBytesBufferNoPool(b *testing.B) {
	payload := []byte("hello world")
	tmp := make([]byte, len(payload))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		buf.Write(payload)
		n, _ := buf.Read(tmp)
		sinkInt = n
	}
}
