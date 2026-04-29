package metrics_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/writefence/writefence/internal/metrics"
)

func TestRegistryIncViolation(t *testing.T) {
	r := metrics.New()
	r.IncViolation("english_only")
	r.IncViolation("english_only")
	r.IncViolation("prefix_required")

	var buf strings.Builder
	r.WritePrometheus(&buf)
	out := buf.String()

	if !strings.Contains(out, `writefence_violations_total{rule="english_only"} 2`) {
		t.Errorf("expected english_only=2, got:\n%s", out)
	}
	if !strings.Contains(out, `writefence_violations_total{rule="prefix_required"} 1`) {
		t.Errorf("expected prefix_required=1, got:\n%s", out)
	}
}

func TestRegistryIncRequest(t *testing.T) {
	r := metrics.New()
	r.IncRequest("documents/text", "allowed")
	r.IncRequest("documents/text", "allowed")
	r.IncRequest("documents/text", "blocked")

	var buf strings.Builder
	r.WritePrometheus(&buf)
	out := buf.String()

	if !strings.Contains(out, `writefence_requests_total{path="documents/text",result="allowed"} 2`) {
		t.Errorf("expected allowed=2, got:\n%s", out)
	}
	if !strings.Contains(out, `writefence_requests_total{path="documents/text",result="blocked"} 1`) {
		t.Errorf("expected blocked=1, got:\n%s", out)
	}
}

func TestRegistryIncDedupMerge(t *testing.T) {
	r := metrics.New()
	r.IncDedupMerge()
	r.IncDedupMerge()
	r.IncDedupMerge()

	var buf strings.Builder
	r.WritePrometheus(&buf)
	out := buf.String()

	if !strings.Contains(out, "writefence_dedup_merges_total 3") {
		t.Errorf("expected dedup_merges=3, got:\n%s", out)
	}
}

func TestRegistryObserveDuration(t *testing.T) {
	r := metrics.New()
	r.ObserveDuration(0.002)
	r.ObserveDuration(0.002)

	var buf strings.Builder
	r.WritePrometheus(&buf)
	out := buf.String()

	// 0.001 bucket: 0 observations <= 0.001
	if !strings.Contains(out, `writefence_request_duration_seconds_bucket{le="0.001"} 0`) {
		t.Errorf("expected 0.001 bucket=0, got:\n%s", out)
	}
	// 0.005 bucket: both 0.002 values <= 0.005
	if !strings.Contains(out, `writefence_request_duration_seconds_bucket{le="0.005"} 2`) {
		t.Errorf("expected 0.005 bucket=2, got:\n%s", out)
	}
	// +Inf bucket: all observations
	if !strings.Contains(out, `writefence_request_duration_seconds_bucket{le="+Inf"} 2`) {
		t.Errorf("expected +Inf bucket=2, got:\n%s", out)
	}
	// count
	if !strings.Contains(out, "writefence_request_duration_seconds_count 2") {
		t.Errorf("expected count=2, got:\n%s", out)
	}
	// sum: 0.002 + 0.002 = 0.004
	if !strings.Contains(out, "writefence_request_duration_seconds_sum 0.004") {
		t.Errorf("expected sum=0.004 in output, got:\n%s", out)
	}
}

func TestRegistryWritePrometheusFormat(t *testing.T) {
	r := metrics.New()
	r.IncViolation("english_only")
	r.IncRequest("documents/text", "allowed")

	var buf strings.Builder
	r.WritePrometheus(&buf)
	out := buf.String()

	// Must contain HELP and TYPE lines
	if !strings.Contains(out, "# HELP writefence_violations_total") {
		t.Error("missing HELP for violations_total")
	}
	if !strings.Contains(out, "# TYPE writefence_violations_total counter") {
		t.Error("missing TYPE for violations_total")
	}
	if !strings.Contains(out, "# HELP writefence_requests_total") {
		t.Error("missing HELP for requests_total")
	}
	if !strings.Contains(out, "# TYPE writefence_requests_total counter") {
		t.Error("missing TYPE for requests_total")
	}
	if !strings.Contains(out, "# HELP writefence_dedup_merges_total") {
		t.Error("missing HELP for dedup_merges_total")
	}
	if !strings.Contains(out, "# HELP writefence_request_duration_seconds") {
		t.Error("missing HELP for request_duration_seconds")
	}
	if !strings.Contains(out, "# TYPE writefence_request_duration_seconds histogram") {
		t.Error("missing TYPE for request_duration_seconds histogram")
	}
	// Must use proper label syntax
	if !strings.Contains(out, `{rule="english_only"}`) {
		t.Error("missing label syntax {rule=\"...\"}")
	}
	// Must be newline-terminated
	if !strings.HasSuffix(out, "\n") {
		t.Error("output must be newline-terminated")
	}
}

func TestRegistryConcurrent(t *testing.T) {
	r := metrics.New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.IncViolation("test")
		}()
	}
	wg.Wait()

	var buf strings.Builder
	r.WritePrometheus(&buf)
	out := buf.String()

	if !strings.Contains(out, `writefence_violations_total{rule="test"} 100`) {
		t.Errorf("expected test=100 after 100 concurrent increments, got:\n%s", out)
	}
}
