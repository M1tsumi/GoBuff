# GoBUFF - High-Performance Buffer Pool

GoBUFF provides a reusable byte buffer and bucketed pool targeting near-zero allocations and sub-50ns operations. It follows the roadmap in `roadmap.txt` with size-class bucketing, calibration, leak detection hooks, and benchmark scaffolding.

## Installation
```bash
go get github.com/your/repo/gobuff
```

## Quick Start
```go
pool := gobuff.NewBufferPool(1024) // default capacity hint

buf := pool.Get()
buf.WriteString("hello")
doWork(buf.Bytes())
pool.Put(buf) // important for reuse
```

## Bucketed Pooling & Calibration
- Buckets default to power-of-two sizes (64..64KiB).
- `GetSized(n)` chooses the closest bucket for `n`.
- Automatic calibration: every `ObserveEvery` puts (default 4096), percentile-based recalibration (default p95, threshold 42000) tunes the default bucket.
- Manual calibration: `Calibrate(observedSize)`.
- `SmallLimit` configures a fast small-buffer sub-pool (default `min(256, smallest bucket)`), reducing overhead for tiny requests.
- `Borrow(n)` returns `(buf, release)` to simplify zero-copy lifetimes.

## Leak Detection (Debug)
Enable finalizer-based leak counting (debug only—avoid in hot paths):
```go
pool := gobuff.NewBufferPoolWithOptions(gobuff.PoolOptions{
    DebugLeakDetection: true,
    SmallLimit:         128, // optional tuning
    Percentile:         0.95,
    CalibrateThreshold: 42000,
})

buf := pool.Get()
// forget to Put(buf)...
runtime.GC()
fmt.Println(pool.LeakCount()) // >0 when leaks are collected
```

## Profiling & Comparisons
- Run pprof on benchmarks:
  ```bash
  go test -bench=BufferPool -benchmem -run=^$ -cpuprofile=cpu.out -memprofile=mem.out
  go tool pprof cpu.out
  ```
- Compare with sync.Pool + bytes.Buffer baselines (built-in benchmarks).
- Compare with bytebufferpool:
  ```bash
  go test -bench=ByteBufferPool -benchmem -tags bytebufferpool
  ```
  (requires `github.com/valyala/bytebufferpool`).

## Benchmarks
Run the built-in benchmarks:
```bash
go test -bench=. -benchmem
```

### Repeatable Benchmark Workflow
Suggested workflow for publishable numbers:
```bash
# 1) baseline
go test -run=^$ -bench=. -benchmem -count=10 > base.txt

# 2) after changes
go test -run=^$ -bench=. -benchmem -count=10 > new.txt

# 3) compare (requires golang.org/x/perf/cmd/benchstat)
benchstat base.txt new.txt
```

Optional competitor comparisons (each requires its own build tag and module):
```bash
go test -run=^$ -bench=ByteBufferPool -benchmem -tags bytebufferpool
go test -run=^$ -bench=BPool -benchmem -tags bpool
go test -run=^$ -bench=VmihailencoBufpool -benchmem -tags bufpool
go test -run=^$ -bench=Libp2p -benchmem -tags libp2pbufferpool
```

## Pre-Benchmark Checklist
- **Build environment**
  - Use a fixed Go version (`go env GOVERSION`) and record it.
  - Ensure CPU frequency scaling/turbo settings are consistent (or at least documented).
  - Close heavy background workloads.
- **Benchmark correctness**
  - Avoid per-iteration allocations in the benchmark harness (e.g. `make([]byte, n)` inside the loop).
  - Use `b.ReportAllocs()` and ensure results reflect the buffer, not the benchmark setup.
  - Consume results via package-level sinks to prevent compiler elimination.
- **Methodology**
  - Run with `-run=^$` to avoid test noise.
  - Run multiple times and compare variance.
  - Prefer `benchstat` for before/after comparisons.
- **Profiles**
  - Capture CPU+mem profiles for the key benchmarks (`-cpuprofile`, `-memprofile`).
  - Validate that improvements reduce allocations/copies in the expected hot paths.
- **Fair comparisons**
  - Compare apples-to-apples: reuse vs non-reuse (`bytes.Buffer` reused vs created per-iteration).
  - Ensure payload sizes and APIs exercised match across implementations.

## Comparable Implementations / References
- **Comparison site**: https://omgnull.github.io/go-benchmark/buffer/
- **valyala/bytebufferpool**: https://github.com/valyala/bytebufferpool
- **libp2p/go-buffer-pool**: https://github.com/libp2p/go-buffer-pool
- **vmihailenco/bufpool**: https://pkg.go.dev/github.com/vmihailenco/bufpool
- **oxtoacart/bpool**: https://github.com/oxtoacart/bpool

Optional comparison vs bytebufferpool (requires dependency and build tag):
```bash
go test -bench=ByteBufferPool -benchmem -tags bytebufferpool
```

## Tests
```bash
go test ./...
```

## License
MIT
