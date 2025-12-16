package gobuff

import (
	"sync"
	"testing"
)

func TestBufferBasicWriteRead(t *testing.T) {
	b := NewBuffer(0)
	if _, err := b.WriteString("hello"); err != nil {
		t.Fatalf("write string: %v", err)
	}
	dst := make([]byte, 5)
	if n, err := b.Read(dst); err != nil {
		t.Fatalf("read: %v", err)
	} else if n != 5 || string(dst) != "hello" {
		t.Fatalf("unexpected read: n=%d, data=%q", n, string(dst))
	}
	if n, err := b.Read(dst); err == nil {
		t.Fatalf("expected EOF, got n=%d", n)
	}
}

func TestBufferPoolReuse(t *testing.T) {
	p := NewBufferPool(0)
	buf := p.Get()
	_, _ = buf.WriteString("data")
	p.Put(buf)

	buf2 := p.Get()
	if got := buf2.String(); got != "" {
		t.Fatalf("expected reset buffer, got %q", got)
	}
}

func TestBufferPoolConcurrent(t *testing.T) {
	const workers = 16
	const iters = 1024
	p := NewBufferPool(8)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				b := p.Get()
				_ = b.WriteByte(byte(j))
				p.Put(b)
			}
		}()
	}
	wg.Wait()
}

func TestBufferPoolSizing(t *testing.T) {
	p := NewBufferPoolWithOptions(PoolOptions{
		BucketSizes: []int{32, 64, 128},
		InitialCap:  32,
	})
	b := p.GetSized(70)
	if cap(b.buf) < 70 {
		t.Fatalf("expected capacity >= 70, got %d", cap(b.buf))
	}
	p.Put(b)
}

func TestBufferPoolCalibrate(t *testing.T) {
	p := NewBufferPoolWithOptions(PoolOptions{
		BucketSizes: []int{16, 32, 64},
	})
	p.Calibrate(40)
	b := p.Get()
	if cap(b.buf) != 64 {
		t.Fatalf("expected calibrated cap 64, got %d", cap(b.buf))
	}
	p.Put(b)
}

func TestBufferPoolStress(t *testing.T) {
	const goroutines = 64
	const iterations = 4096
	p := NewBufferPoolWithOptions(PoolOptions{
		BucketSizes:  []int{64, 256, 1024, 4096},
		ObserveEvery: 512,
	})

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				size := (j + id) % 3000
				buf := p.GetSized(size)
				for k := 0; k < 3; k++ {
					_ = buf.WriteByte(byte(k))
				}
				p.Put(buf)
			}
		}(i)
	}
	wg.Wait()
	if leaks := p.LeakCount(); leaks != 0 && !p.debugLeaks {
		t.Fatalf("unexpected leaks reported: %d", leaks)
	}
}
