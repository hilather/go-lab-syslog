package observability

import (
	"strconv"
	"sync"
)

// Registry holds OBS-001 extra counters that are not ingest/store gauges.
type Registry struct {
	mu           sync.Mutex
	waitTimeouts uint64
	apply        map[string]uint64
	http         map[httpKey]uint64
}

type httpKey struct {
	code  int
	route string
}

// HTTPSample is one observed {code,route} counter.
type HTTPSample struct {
	Code  int
	Route string
	Value uint64
}

// NewRegistry constructs extra counters.
func NewRegistry() *Registry {
	return &Registry{
		apply: map[string]uint64{},
		http:  map[httpKey]uint64{},
	}
}

// IncWaitTimeout records one wait_timeout.
func (r *Registry) IncWaitTimeout() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.waitTimeouts++
	r.mu.Unlock()
}

// WaitTimeouts is the wait_timeout counter.
func (r *Registry) WaitTimeouts() uint64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.waitTimeouts
}

// IncApply records one changes:apply result label.
func (r *Registry) IncApply(result string) {
	if r == nil {
		return
	}
	if result == "" {
		result = "error"
	}
	r.mu.Lock()
	if r.apply == nil {
		r.apply = map[string]uint64{}
	}
	r.apply[result]++
	r.mu.Unlock()
}

// ApplyByResult copies apply counters.
func (r *Registry) ApplyByResult() map[string]uint64 {
	if r == nil {
		return map[string]uint64{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]uint64, len(r.apply))
	for k, v := range r.apply {
		out[k] = v
	}
	return out
}

// IncHTTP records one management HTTP response. No client-IP labels.
func (r *Registry) IncHTTP(code int, route string) {
	if r == nil {
		return
	}
	if route == "" {
		route = "unknown"
	}
	r.mu.Lock()
	if r.http == nil {
		r.http = map[httpKey]uint64{}
	}
	r.http[httpKey{code: code, route: route}]++
	r.mu.Unlock()
}

// HTTPRequests copies observed HTTP counters.
func (r *Registry) HTTPRequests() []HTTPSample {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]HTTPSample, 0, len(r.http))
	for k, v := range r.http {
		out = append(out, HTTPSample{Code: k.code, Route: k.route, Value: v})
	}
	return out
}

func formatCode(code int) string {
	return strconv.Itoa(code)
}
