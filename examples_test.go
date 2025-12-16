package gobuff

import (
	"fmt"
	"io"
)

func ExampleBuffer_usage() {
	b := NewBuffer(16)
	_, _ = b.WriteString("hello")
	out := make([]byte, 5)
	_, _ = b.Read(out)
	fmt.Println(string(out))
	// Output: hello
}

func ExampleBuffer_reserve() {
	b := NewBuffer(0)
	s := b.Reserve(3)
	copy(s, []byte{1, 2, 3})
	fmt.Println(b.Bytes())
	// Output: [1 2 3]
}

func ExampleBufferPool_Borrow() {
	pool := NewBufferPoolWithOptions(PoolOptions{
		InitialCap: 64,
	})
	buf, release := pool.Borrow(32)
	_, _ = buf.WriteString("data")
	_, _ = buf.WriteTo(io.Discard)
	release()
	stats := pool.Stats()
	fmt.Printf("gets=%d puts=%d", stats.Gets, stats.Puts)
	// Output: gets=1 puts=1
}
