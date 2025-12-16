package gobuff

import (
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
)

var defaultBucketSizes = []int{64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536}

const cacheLineSize = 64
const defaultPercentile = 0.95
const defaultCalibrateThreshold = 42000

func (p *BufferPool) _keepPadding() {
	_ = cacheLineSize
	_ = p._pad0
	_ = p._pad1
}

// BufferPool wraps multiple sync.Pool buckets to reuse Buffer instances with sizing hints.
// It supports:
//   - Bucketed pools for size classes.
//   - Auto-calibration of the default bucket based on observed usage.
//   - Optional leak detection via finalizers (debug only; avoid in hot paths).
type BufferPool struct {
	sizes        []int
	buckets      []sync.Pool
	smallPool    sync.Pool
	_pad0        [cacheLineSize]byte // isolate pools from counters
	defaultCap   atomic.Int64
	observeEvery int64
	observed     atomic.Int64
	bucketHits   []atomic.Int64
	percentile   float64
	calibrateThr int64
	_pad1        [cacheLineSize]byte // isolate counters from stats
	smallLimit   int
	debugLeaks   bool
	leaks        atomic.Int64
	gets         atomic.Int64
	puts         atomic.Int64
	allocs       atomic.Int64
	calibrations atomic.Int64
	metrics      func(Stats)
}

// PoolOptions configures a BufferPool.
type PoolOptions struct {
	// BucketSizes allows overriding default power-of-two buckets.
	// Values must be positive; they will be sorted and de-duplicated.
	BucketSizes []int
	// InitialCap sets the default capacity for Get().
	InitialCap int
	// DebugLeakDetection enables runtime finalizers that count leaked buffers.
	DebugLeakDetection bool
	// ObserveEvery controls how many Put operations are sampled before auto-calibration runs.
	// If zero or negative, a default of 4096 is used.
	ObserveEvery int
	// SmallLimit configures the cutoff (in bytes) for the fast small-buffer pool.
	// If zero or negative, a default based on the smallest bucket is used (min(256, smallest bucket)).
	SmallLimit int
	// Percentile selects the target percentile for calibration (0-1). Default 0.95.
	Percentile float64
	// CalibrateThreshold sets the number of observed puts before percentile calibration. Default 42000.
	CalibrateThreshold int64
	// Metrics, if provided, is invoked on calibration with a snapshot of Stats.
	Metrics func(Stats)
}

// NewBufferPool initializes a pool that produces empty Buffers with the given initial capacity.
// Use initialCap = 0 for default buckets.
func NewBufferPool(initialCap int) *BufferPool {
	return NewBufferPoolWithOptions(PoolOptions{InitialCap: initialCap})
}

// NewBufferPoolWithOptions constructs a BufferPool with optional bucket sizing and leak detection.
func NewBufferPoolWithOptions(opts PoolOptions) *BufferPool {
	sizes := normalizeSizes(opts.BucketSizes)
	if len(sizes) == 0 {
		sizes = append([]int(nil), defaultBucketSizes...)
	}

	p := &BufferPool{
		sizes:        sizes,
		defaultCap:   atomic.Int64{},
		debugLeaks:   opts.DebugLeakDetection,
		observeEvery: 4096,
		bucketHits:   make([]atomic.Int64, len(sizes)),
		percentile:   defaultPercentile,
		calibrateThr: defaultCalibrateThreshold,
		metrics:      opts.Metrics,
	}
	p._keepPadding()
	if opts.ObserveEvery > 0 {
		p.observeEvery = int64(opts.ObserveEvery)
	}
	if opts.Percentile > 0 && opts.Percentile <= 1 {
		p.percentile = opts.Percentile
	}
	if opts.CalibrateThreshold > 0 {
		p.calibrateThr = opts.CalibrateThreshold
	}
	p.smallLimit = minInt(256, sizes[0])
	if opts.SmallLimit > 0 {
		p.smallLimit = opts.SmallLimit
	}
	p.defaultCap.Store(int64(chooseCap(sizes, opts.InitialCap)))

	p.buckets = make([]sync.Pool, len(sizes))
	for i, size := range sizes {
		capacity := size
		p.buckets[i] = sync.Pool{
			New: func() any {
				p.allocs.Add(1)
				return NewBuffer(capacity)
			},
		}
	}
	p.smallPool = sync.Pool{
		New: func() any {
			p.allocs.Add(1)
			return NewBuffer(p.smallLimit)
		},
	}
	return p
}

// Get retrieves a Buffer using the pool's default capacity.
func (p *BufferPool) Get() *Buffer {
	p.gets.Add(1)
	return p.getSized(int(p.defaultCap.Load()))
}

// GetSized retrieves a Buffer sized for n bytes using bucketed pools.
func (p *BufferPool) GetSized(n int) *Buffer {
	p.gets.Add(1)
	return p.getSized(n)
}

// Borrow returns a buffer and a release function that must be called to return it to the pool.
// This is useful for zero-copy workflows while keeping lifetime management explicit.
func (p *BufferPool) Borrow(n int) (*Buffer, func()) {
	p.gets.Add(1)
	buf := p.getSized(n)
	return buf, func() { p.Put(buf) }
}

// Calibrate adjusts the default capacity to the nearest bucket for the observed size.
func (p *BufferPool) Calibrate(observed int) {
	if observed <= 0 {
		return
	}
	p.defaultCap.Store(int64(chooseCap(p.sizes, observed)))
	p.calibrations.Add(1)
	if p.metrics != nil {
		p.metrics(p.Stats())
	}
}

// LeakCount returns the number of buffers that were garbage-collected without being returned when leak detection is enabled.
func (p *BufferPool) LeakCount() int64 {
	return p.leaks.Load()
}

// Put resets and returns the Buffer to an appropriate bucket.
func (p *BufferPool) Put(b *Buffer) {
	if b == nil {
		return
	}
	p.puts.Add(1)
	if p.debugLeaks {
		runtime.SetFinalizer(b, nil)
	}
	b.Reset()
	if cap(b.buf) <= p.smallLimit {
		idx := p.bucketIndex(cap(b.buf))
		p.observeSize(cap(b.buf), idx)
		p.smallPool.Put(b)
		return
	}
	idx := p.bucketIndex(cap(b.buf))
	p.observeSize(cap(b.buf), idx)
	p.buckets[idx].Put(b)
}

func (p *BufferPool) getSized(n int) *Buffer {
	if n < 0 {
		n = 0
	}
	if n <= p.smallLimit {
		buf := p.smallPool.Get().(*Buffer)
		if n > cap(buf.buf) {
			buf.grow(n - len(buf.buf))
		}
		if p.debugLeaks {
			runtime.SetFinalizer(buf, func(_ *Buffer) {
				p.leaks.Add(1)
			})
		}
		return buf
	}
	idx := p.bucketIndex(n)
	buf := p.buckets[idx].Get().(*Buffer)
	// If the buffer is too small for the requested size (possible when n exceeds largest bucket),
	// grow it to fit.
	if n > cap(buf.buf) {
		buf.grow(n - len(buf.buf))
	}
	if p.debugLeaks {
		runtime.SetFinalizer(buf, func(_ *Buffer) {
			p.leaks.Add(1)
		})
	}
	return buf
}

func (p *BufferPool) bucketIndex(size int) int {
	if size <= 0 {
		return 0
	}
	for i, s := range p.sizes {
		if size <= s {
			return i
		}
	}
	return len(p.sizes) - 1
}

func chooseCap(sizes []int, target int) int {
	if target <= 0 {
		return sizes[0]
	}
	for _, s := range sizes {
		if target <= s {
			return s
		}
	}
	return sizes[len(sizes)-1]
}

func normalizeSizes(s []int) []int {
	var filtered []int
	for _, v := range s {
		if v > 0 {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		return filtered
	}
	sort.Ints(filtered)
	uniq := filtered[:1]
	for i := 1; i < len(filtered); i++ {
		if filtered[i] != uniq[len(uniq)-1] {
			uniq = append(uniq, filtered[i])
		}
	}
	return uniq
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *BufferPool) observeSize(size int, bucketIdx int) {
	if size <= 0 || p.observeEvery <= 0 {
		return
	}
	p.bucketHits[bucketIdx].Add(1)
	total := p.observed.Add(1)
	if total%p.observeEvery != 0 {
		return
	}
	p.recalibratePercentile()
}

func (p *BufferPool) recalibratePercentile() {
	// Collect counts and total
	var total int64
	counts := make([]int64, len(p.bucketHits))
	for i := range p.bucketHits {
		counts[i] = p.bucketHits[i].Swap(0)
		total += counts[i]
	}
	if total <= 0 || total < p.calibrateThr {
		return
	}
	target := int64(float64(total) * p.percentile)
	if target <= 0 {
		target = total
	}
	var cumulative int64
	for i, c := range counts {
		cumulative += c
		if cumulative >= target {
			p.defaultCap.Store(int64(p.sizes[i]))
			p.calibrations.Add(1)
			if p.metrics != nil {
				p.metrics(p.Stats())
			}
			return
		}
	}
}

// Stats provides counters for observability.
type Stats struct {
	Gets         int64
	Puts         int64
	Allocs       int64
	Calibrations int64
	LeakCount    int64
	DefaultCap   int64
	SmallLimit   int
}

// Stats returns a snapshot of pool counters.
func (p *BufferPool) Stats() Stats {
	return Stats{
		Gets:         p.gets.Load(),
		Puts:         p.puts.Load(),
		Allocs:       p.allocs.Load(),
		Calibrations: p.calibrations.Load(),
		LeakCount:    p.leaks.Load(),
		DefaultCap:   p.defaultCap.Load(),
		SmallLimit:   p.smallLimit,
	}
}
