// Package metrics provides thread-safe Prometheus-compatible metrics for WriteFence.
// It uses stdlib only — no external dependencies.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// prometheusEscape escapes a string for use as a Prometheus label value.
func prometheusEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// histogramBuckets are the upper bounds (in seconds) for request duration buckets.
var histogramBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1}

// labeledCounter is a thread-safe counter keyed by a label string.
type labeledCounter struct {
	mu sync.Mutex
	m  map[string]int64
}

func newLabeledCounter() *labeledCounter {
	return &labeledCounter{m: make(map[string]int64)}
}

func (c *labeledCounter) inc(key string) {
	c.mu.Lock()
	c.m[key]++
	c.mu.Unlock()
}

func (c *labeledCounter) snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make(map[string]int64, len(c.m))
	for k, v := range c.m {
		cp[k] = v
	}
	return cp
}

// histogram tracks observed durations in pre-defined buckets.
type histogram struct {
	mu      sync.Mutex
	buckets map[float64]int64 // upper_bound → cumulative count
	sum     float64
	count   int64
}

func newHistogram() *histogram {
	b := make(map[float64]int64, len(histogramBuckets))
	for _, le := range histogramBuckets {
		b[le] = 0
	}
	return &histogram{buckets: b}
}

func (h *histogram) observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sum += v
	h.count++
	for _, le := range histogramBuckets {
		if v <= le {
			h.buckets[le]++
		}
	}
}

// Registry holds all WriteFence metrics. Create one with New() and pass it around.
type Registry struct {
	violations  *labeledCounter // writefence_violations_total{rule="..."}
	requests    *labeledCounter // writefence_requests_total{path="...",result="..."}
	dedupMerges int64           // writefence_dedup_merges_total (atomic)
	duration    *histogram      // writefence_request_duration_seconds
}

// New creates and returns a new, zeroed Registry.
func New() *Registry {
	return &Registry{
		violations: newLabeledCounter(),
		requests:   newLabeledCounter(),
		duration:   newHistogram(),
	}
}

// IncViolation increments writefence_violations_total{rule=rule}.
func (r *Registry) IncViolation(rule string) {
	r.violations.inc(rule)
}

// IncRequest increments writefence_requests_total{path=path,result=result}.
func (r *Registry) IncRequest(path, result string) {
	r.requests.inc(path + "\x00" + result)
}

// IncDedupMerge increments writefence_dedup_merges_total.
func (r *Registry) IncDedupMerge() {
	atomic.AddInt64(&r.dedupMerges, 1)
}

// ObserveDuration records a single duration observation in seconds.
func (r *Registry) ObserveDuration(seconds float64) {
	r.duration.observe(seconds)
}

// WritePrometheus writes all metrics in the Prometheus text exposition format to w.
func (r *Registry) WritePrometheus(w io.Writer) {
	// --- writefence_violations_total ---
	fmt.Fprint(w, "# HELP writefence_violations_total Total number of blocked document writes by rule\n")
	fmt.Fprint(w, "# TYPE writefence_violations_total counter\n")
	vSnap := r.violations.snapshot()
	vKeys := sortedKeys(vSnap)
	for _, rule := range vKeys {
		fmt.Fprintf(w, "writefence_violations_total{rule=\"%s\"} %d\n", prometheusEscape(rule), vSnap[rule])
	}

	// --- writefence_requests_total ---
	fmt.Fprint(w, "# HELP writefence_requests_total Total document write requests by path and result\n")
	fmt.Fprint(w, "# TYPE writefence_requests_total counter\n")
	rSnap := r.requests.snapshot()
	rKeys := sortedKeys(rSnap)
	for _, key := range rKeys {
		path, result := splitRequestKey(key)
		fmt.Fprintf(w, "writefence_requests_total{path=\"%s\",result=\"%s\"} %d\n", prometheusEscape(path), prometheusEscape(result), rSnap[key])
	}

	// --- writefence_dedup_merges_total ---
	fmt.Fprint(w, "# HELP writefence_dedup_merges_total Total number of STATUS dedup merges performed\n")
	fmt.Fprint(w, "# TYPE writefence_dedup_merges_total counter\n")
	fmt.Fprintf(w, "writefence_dedup_merges_total %d\n", atomic.LoadInt64(&r.dedupMerges))

	// --- writefence_request_duration_seconds ---
	fmt.Fprint(w, "# HELP writefence_request_duration_seconds Rule evaluation duration in seconds\n")
	fmt.Fprint(w, "# TYPE writefence_request_duration_seconds histogram\n")
	r.duration.mu.Lock()
	dSum := r.duration.sum
	dCount := r.duration.count
	// Copy bucket counts while holding the lock.
	bucketCounts := make(map[float64]int64, len(histogramBuckets))
	for le, cnt := range r.duration.buckets {
		bucketCounts[le] = cnt
	}
	r.duration.mu.Unlock()

	// Write buckets in sorted order, then +Inf.
	for _, le := range histogramBuckets {
		fmt.Fprintf(w, "writefence_request_duration_seconds_bucket{le=\"%g\"} %d\n", le, bucketCounts[le])
	}
	fmt.Fprintf(w, "writefence_request_duration_seconds_bucket{le=\"+Inf\"} %d\n", dCount)
	fmt.Fprintf(w, "writefence_request_duration_seconds_sum %g\n", dSum)
	fmt.Fprintf(w, "writefence_request_duration_seconds_count %d\n", dCount)
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// splitRequestKey splits the composite key used in the requests counter.
// Format: path + "\x00" + result
func splitRequestKey(key string) (path, result string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0x00 {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
