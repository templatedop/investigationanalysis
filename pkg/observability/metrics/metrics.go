// Package metrics provides Prometheus metrics for the fraud investigation system
package metrics

import (
	"net/http"
	"sync"
	"time"
)

// MetricsConfig configures metrics collection
type MetricsConfig struct {
	Enabled     bool   `json:"enabled"`
	Endpoint    string `json:"endpoint"`
	Namespace   string `json:"namespace"`
	Subsystem   string `json:"subsystem"`
	EnableGo    bool   `json:"enable_go_metrics"`
	EnableProc  bool   `json:"enable_process_metrics"`
}

// DefaultMetricsConfig returns default configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:     true,
		Endpoint:    "/metrics",
		Namespace:   "fraud_investigation",
		Subsystem:   "",
		EnableGo:    true,
		EnableProc:  true,
	}
}

// Metric types
type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeGauge
	MetricTypeHistogram
	MetricTypeSummary
)

// Labels represents metric labels
type Labels map[string]string

// Metric interface
type Metric interface {
	Name() string
	Type() MetricType
	Help() string
	Labels() []string
}

// Counter is a monotonically increasing counter
type Counter struct {
	name   string
	help   string
	labels []string
	values map[string]float64
	mu     sync.RWMutex
}

// NewCounter creates a new counter
func NewCounter(name, help string, labels ...string) *Counter {
	return &Counter{
		name:   name,
		help:   help,
		labels: labels,
		values: make(map[string]float64),
	}
}

func (c *Counter) Name() string      { return c.name }
func (c *Counter) Type() MetricType  { return MetricTypeCounter }
func (c *Counter) Help() string      { return c.help }
func (c *Counter) Labels() []string  { return c.labels }

// Inc increments the counter by 1
func (c *Counter) Inc(labels Labels) {
	c.Add(1, labels)
}

// Add adds a value to the counter
func (c *Counter) Add(v float64, labels Labels) {
	key := labelsToKey(labels)
	c.mu.Lock()
	c.values[key] += v
	c.mu.Unlock()
}

// Value returns current counter value
func (c *Counter) Value(labels Labels) float64 {
	key := labelsToKey(labels)
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.values[key]
}

// Gauge is a metric that can go up or down
type Gauge struct {
	name   string
	help   string
	labels []string
	values map[string]float64
	mu     sync.RWMutex
}

// NewGauge creates a new gauge
func NewGauge(name, help string, labels ...string) *Gauge {
	return &Gauge{
		name:   name,
		help:   help,
		labels: labels,
		values: make(map[string]float64),
	}
}

func (g *Gauge) Name() string      { return g.name }
func (g *Gauge) Type() MetricType  { return MetricTypeGauge }
func (g *Gauge) Help() string      { return g.help }
func (g *Gauge) Labels() []string  { return g.labels }

// Set sets the gauge value
func (g *Gauge) Set(v float64, labels Labels) {
	key := labelsToKey(labels)
	g.mu.Lock()
	g.values[key] = v
	g.mu.Unlock()
}

// Inc increments the gauge by 1
func (g *Gauge) Inc(labels Labels) {
	g.Add(1, labels)
}

// Dec decrements the gauge by 1
func (g *Gauge) Dec(labels Labels) {
	g.Add(-1, labels)
}

// Add adds a value to the gauge
func (g *Gauge) Add(v float64, labels Labels) {
	key := labelsToKey(labels)
	g.mu.Lock()
	g.values[key] += v
	g.mu.Unlock()
}

// Value returns current gauge value
func (g *Gauge) Value(labels Labels) float64 {
	key := labelsToKey(labels)
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.values[key]
}

// Histogram tracks distribution of values
type Histogram struct {
	name    string
	help    string
	labels  []string
	buckets []float64
	counts  map[string][]uint64
	sums    map[string]float64
	totals  map[string]uint64
	mu      sync.RWMutex
}

// NewHistogram creates a new histogram
func NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	return &Histogram{
		name:    name,
		help:    help,
		labels:  labels,
		buckets: buckets,
		counts:  make(map[string][]uint64),
		sums:    make(map[string]float64),
		totals:  make(map[string]uint64),
	}
}

// DefaultBuckets returns default histogram buckets
func DefaultBuckets() []float64 {
	return []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
}

func (h *Histogram) Name() string      { return h.name }
func (h *Histogram) Type() MetricType  { return MetricTypeHistogram }
func (h *Histogram) Help() string      { return h.help }
func (h *Histogram) Labels() []string  { return h.labels }

// Observe records a value
func (h *Histogram) Observe(v float64, labels Labels) {
	key := labelsToKey(labels)
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.counts[key]; !ok {
		h.counts[key] = make([]uint64, len(h.buckets))
	}

	for i, bucket := range h.buckets {
		if v <= bucket {
			h.counts[key][i]++
		}
	}

	h.sums[key] += v
	h.totals[key]++
}

// Timer is a helper for timing operations
type Timer struct {
	histogram *Histogram
	labels    Labels
	start     time.Time
}

// NewTimer creates a new timer
func (h *Histogram) NewTimer(labels Labels) *Timer {
	return &Timer{
		histogram: h,
		labels:    labels,
		start:     time.Now(),
	}
}

// ObserveDuration records the duration
func (t *Timer) ObserveDuration() time.Duration {
	d := time.Since(t.start)
	t.histogram.Observe(d.Seconds(), t.labels)
	return d
}

// Registry holds all registered metrics
type Registry struct {
	metrics  map[string]Metric
	counters map[string]*Counter
	gauges   map[string]*Gauge
	hists    map[string]*Histogram
	mu       sync.RWMutex
	config   *MetricsConfig
}

var (
	defaultRegistry *Registry
	registryOnce    sync.Once
)

// NewRegistry creates a new metrics registry
func NewRegistry(config *MetricsConfig) *Registry {
	return &Registry{
		metrics:  make(map[string]Metric),
		counters: make(map[string]*Counter),
		gauges:   make(map[string]*Gauge),
		hists:    make(map[string]*Histogram),
		config:   config,
	}
}

// Default returns the default registry
func Default() *Registry {
	registryOnce.Do(func() {
		defaultRegistry = NewRegistry(DefaultMetricsConfig())
	})
	return defaultRegistry
}

// SetDefault sets the default registry
func SetDefault(r *Registry) {
	defaultRegistry = r
}

// Register registers a metric
func (r *Registry) Register(m Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics[m.Name()] = m
}

// RegisterCounter registers a counter
func (r *Registry) RegisterCounter(name, help string, labels ...string) *Counter {
	c := NewCounter(r.prefixName(name), help, labels...)
	r.mu.Lock()
	r.counters[name] = c
	r.metrics[name] = c
	r.mu.Unlock()
	return c
}

// RegisterGauge registers a gauge
func (r *Registry) RegisterGauge(name, help string, labels ...string) *Gauge {
	g := NewGauge(r.prefixName(name), help, labels...)
	r.mu.Lock()
	r.gauges[name] = g
	r.metrics[name] = g
	r.mu.Unlock()
	return g
}

// RegisterHistogram registers a histogram
func (r *Registry) RegisterHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	h := NewHistogram(r.prefixName(name), help, buckets, labels...)
	r.mu.Lock()
	r.hists[name] = h
	r.metrics[name] = h
	r.mu.Unlock()
	return h
}

// Counter returns a registered counter
func (r *Registry) Counter(name string) *Counter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.counters[name]
}

// Gauge returns a registered gauge
func (r *Registry) Gauge(name string) *Gauge {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.gauges[name]
}

// Histogram returns a registered histogram
func (r *Registry) Histogram(name string) *Histogram {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hists[name]
}

// prefixName adds namespace and subsystem prefix
func (r *Registry) prefixName(name string) string {
	if r.config.Namespace != "" {
		name = r.config.Namespace + "_" + name
	}
	if r.config.Subsystem != "" {
		name = r.config.Subsystem + "_" + name
	}
	return name
}

// Handler returns an HTTP handler for metrics
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.RLock()
		defer r.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		for _, m := range r.metrics {
			writeMetric(w, m)
		}
	})
}

func writeMetric(w http.ResponseWriter, m Metric) {
	// Write help line
	w.Write([]byte("# HELP " + m.Name() + " " + m.Help() + "\n"))

	// Write type line
	var typeStr string
	switch m.Type() {
	case MetricTypeCounter:
		typeStr = "counter"
	case MetricTypeGauge:
		typeStr = "gauge"
	case MetricTypeHistogram:
		typeStr = "histogram"
	case MetricTypeSummary:
		typeStr = "summary"
	}
	w.Write([]byte("# TYPE " + m.Name() + " " + typeStr + "\n"))

	// Write values based on type
	switch v := m.(type) {
	case *Counter:
		v.mu.RLock()
		for key, val := range v.values {
			w.Write([]byte(m.Name() + key + " " + formatFloat(val) + "\n"))
		}
		v.mu.RUnlock()
	case *Gauge:
		v.mu.RLock()
		for key, val := range v.values {
			w.Write([]byte(m.Name() + key + " " + formatFloat(val) + "\n"))
		}
		v.mu.RUnlock()
	case *Histogram:
		v.mu.RLock()
		for key, counts := range v.counts {
			for i, count := range counts {
				bucket := formatFloat(v.buckets[i])
				w.Write([]byte(m.Name() + "_bucket{le=\"" + bucket + "\"" + key + "} " + formatUint(count) + "\n"))
			}
			w.Write([]byte(m.Name() + "_sum" + key + " " + formatFloat(v.sums[key]) + "\n"))
			w.Write([]byte(m.Name() + "_count" + key + " " + formatUint(v.totals[key]) + "\n"))
		}
		v.mu.RUnlock()
	}
}

func labelsToKey(labels Labels) string {
	if len(labels) == 0 {
		return ""
	}
	key := "{"
	first := true
	for k, v := range labels {
		if !first {
			key += ","
		}
		key += k + "=\"" + v + "\""
		first = false
	}
	key += "}"
	return key
}

func formatFloat(v float64) string {
	return string(append([]byte{}, []byte{}...)) + formatNumber(v)
}

func formatUint(v uint64) string {
	if v == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	return string(buf)
}

func formatNumber(v float64) string {
	// Simple float formatting
	if v == 0 {
		return "0"
	}

	intPart := int64(v)
	fracPart := v - float64(intPart)

	result := formatInt64(intPart)
	if fracPart != 0 {
		result += "."
		fracPart *= 1000000
		if fracPart < 0 {
			fracPart = -fracPart
		}
		frac := formatUint(uint64(fracPart))
		// Pad with leading zeros
		for len(frac) < 6 {
			frac = "0" + frac
		}
		// Trim trailing zeros
		for len(frac) > 1 && frac[len(frac)-1] == '0' {
			frac = frac[:len(frac)-1]
		}
		result += frac
	}
	return result
}

func formatInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := make([]byte, 0, 20)
	for v > 0 {
		buf = append([]byte{byte('0' + v%10)}, buf...)
		v /= 10
	}
	if neg {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

// Standard metrics for fraud investigation

// InvestigationMetrics holds investigation-related metrics
type InvestigationMetrics struct {
	InvestigationsTotal   *Counter
	InvestigationsActive  *Gauge
	InvestigationDuration *Histogram
	RiskScores            *Histogram
	DecisionsTotal        *Counter
	EscalationsTotal      *Counter
	AgentExecutions       *Counter
	AgentDuration         *Histogram
	AgentErrors           *Counter
	RAGQueries            *Counter
	RAGLatency            *Histogram
}

// NewInvestigationMetrics creates standard investigation metrics
func NewInvestigationMetrics(registry *Registry) *InvestigationMetrics {
	return &InvestigationMetrics{
		InvestigationsTotal: registry.RegisterCounter(
			"investigations_total",
			"Total number of investigations",
			"status",
		),
		InvestigationsActive: registry.RegisterGauge(
			"investigations_active",
			"Number of active investigations",
		),
		InvestigationDuration: registry.RegisterHistogram(
			"investigation_duration_seconds",
			"Duration of investigations",
			[]float64{1, 5, 10, 30, 60, 120, 300, 600},
			"status",
		),
		RiskScores: registry.RegisterHistogram(
			"risk_scores",
			"Distribution of risk scores",
			[]float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			"risk_level",
		),
		DecisionsTotal: registry.RegisterCounter(
			"decisions_total",
			"Total decisions made",
			"decision", "method",
		),
		EscalationsTotal: registry.RegisterCounter(
			"escalations_total",
			"Total escalations",
			"reason",
		),
		AgentExecutions: registry.RegisterCounter(
			"agent_executions_total",
			"Total agent executions",
			"agent", "status",
		),
		AgentDuration: registry.RegisterHistogram(
			"agent_duration_seconds",
			"Duration of agent executions",
			DefaultBuckets(),
			"agent",
		),
		AgentErrors: registry.RegisterCounter(
			"agent_errors_total",
			"Total agent errors",
			"agent", "error_type",
		),
		RAGQueries: registry.RegisterCounter(
			"rag_queries_total",
			"Total RAG queries",
			"store",
		),
		RAGLatency: registry.RegisterHistogram(
			"rag_latency_seconds",
			"RAG query latency",
			[]float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
			"store",
		),
	}
}

// RecordInvestigation records investigation metrics
func (m *InvestigationMetrics) RecordInvestigation(status string, duration time.Duration) {
	m.InvestigationsTotal.Inc(Labels{"status": status})
	m.InvestigationDuration.Observe(duration.Seconds(), Labels{"status": status})
}

// RecordRiskScore records a risk score
func (m *InvestigationMetrics) RecordRiskScore(score float64, level string) {
	m.RiskScores.Observe(score, Labels{"risk_level": level})
}

// RecordDecision records a decision
func (m *InvestigationMetrics) RecordDecision(decision, method string) {
	m.DecisionsTotal.Inc(Labels{"decision": decision, "method": method})
}

// RecordEscalation records an escalation
func (m *InvestigationMetrics) RecordEscalation(reason string) {
	m.EscalationsTotal.Inc(Labels{"reason": reason})
}

// RecordAgentExecution records agent execution
func (m *InvestigationMetrics) RecordAgentExecution(agent, status string, duration time.Duration) {
	m.AgentExecutions.Inc(Labels{"agent": agent, "status": status})
	m.AgentDuration.Observe(duration.Seconds(), Labels{"agent": agent})
}

// RecordAgentError records agent error
func (m *InvestigationMetrics) RecordAgentError(agent, errorType string) {
	m.AgentErrors.Inc(Labels{"agent": agent, "error_type": errorType})
}

// RecordRAGQuery records RAG query
func (m *InvestigationMetrics) RecordRAGQuery(store string, duration time.Duration) {
	m.RAGQueries.Inc(Labels{"store": store})
	m.RAGLatency.Observe(duration.Seconds(), Labels{"store": store})
}
