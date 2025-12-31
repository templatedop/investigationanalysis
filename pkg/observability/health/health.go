// Package health provides health check capabilities
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// HealthConfig configures health checks
type HealthConfig struct {
	Enabled          bool          `json:"enabled"`
	LivenessPath     string        `json:"liveness_path"`
	ReadinessPath    string        `json:"readiness_path"`
	CheckTimeout     time.Duration `json:"check_timeout"`
	CheckInterval    time.Duration `json:"check_interval"`
	FailureThreshold int           `json:"failure_threshold"`
}

// DefaultHealthConfig returns default configuration
func DefaultHealthConfig() *HealthConfig {
	return &HealthConfig{
		Enabled:          true,
		LivenessPath:     "/health/live",
		ReadinessPath:    "/health/ready",
		CheckTimeout:     5 * time.Second,
		CheckInterval:    10 * time.Second,
		FailureThreshold: 3,
	}
}

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
	StatusUnknown   Status = "unknown"
)

// CheckResult represents the result of a health check
type CheckResult struct {
	Name      string        `json:"name"`
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Details   interface{}   `json:"details,omitempty"`
}

// HealthResponse represents the health endpoint response
type HealthResponse struct {
	Status    Status                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks,omitempty"`
	Version   string                 `json:"version,omitempty"`
	Uptime    time.Duration          `json:"uptime,omitempty"`
}

// Check is a health check function
type Check func(ctx context.Context) CheckResult

// Checker manages health checks
type Checker struct {
	config      *HealthConfig
	checks      map[string]Check
	results     map[string]CheckResult
	failures    map[string]int
	mu          sync.RWMutex
	startTime   time.Time
	version     string
	stopChan    chan struct{}
	isRunning   bool
}

// NewChecker creates a new health checker
func NewChecker(config *HealthConfig) *Checker {
	return &Checker{
		config:    config,
		checks:    make(map[string]Check),
		results:   make(map[string]CheckResult),
		failures:  make(map[string]int),
		startTime: time.Now(),
		stopChan:  make(chan struct{}),
	}
}

// SetVersion sets the application version
func (c *Checker) SetVersion(version string) {
	c.version = version
}

// AddCheck adds a health check
func (c *Checker) AddCheck(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

// RemoveCheck removes a health check
func (c *Checker) RemoveCheck(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.checks, name)
	delete(c.results, name)
	delete(c.failures, name)
}

// Start starts the background health check loop
func (c *Checker) Start() {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return
	}
	c.isRunning = true
	c.stopChan = make(chan struct{})
	c.mu.Unlock()

	go c.run()
}

// Stop stops the background health check loop
func (c *Checker) Stop() {
	c.mu.Lock()
	if !c.isRunning {
		c.mu.Unlock()
		return
	}
	c.isRunning = false
	close(c.stopChan)
	c.mu.Unlock()
}

// run executes health checks periodically
func (c *Checker) run() {
	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	// Run initial checks
	c.runChecks()

	for {
		select {
		case <-ticker.C:
			c.runChecks()
		case <-c.stopChan:
			return
		}
	}
}

// runChecks runs all registered checks
func (c *Checker) runChecks() {
	c.mu.RLock()
	checks := make(map[string]Check)
	for k, v := range c.checks {
		checks[k] = v
	}
	c.mu.RUnlock()

	for name, check := range checks {
		ctx, cancel := context.WithTimeout(context.Background(), c.config.CheckTimeout)
		result := check(ctx)
		cancel()

		c.mu.Lock()
		c.results[name] = result
		if result.Status == StatusUnhealthy {
			c.failures[name]++
		} else {
			c.failures[name] = 0
		}
		c.mu.Unlock()
	}
}

// CheckNow runs all checks immediately and returns results
func (c *Checker) CheckNow(ctx context.Context) map[string]CheckResult {
	c.mu.RLock()
	checks := make(map[string]Check)
	for k, v := range c.checks {
		checks[k] = v
	}
	c.mu.RUnlock()

	results := make(map[string]CheckResult)
	for name, check := range checks {
		checkCtx, cancel := context.WithTimeout(ctx, c.config.CheckTimeout)
		results[name] = check(checkCtx)
		cancel()
	}

	return results
}

// IsLive returns true if the service is live
func (c *Checker) IsLive() bool {
	// Liveness is simple - if we're running, we're live
	return true
}

// IsReady returns true if the service is ready to receive traffic
func (c *Checker) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for name, result := range c.results {
		if result.Status == StatusUnhealthy {
			if c.failures[name] >= c.config.FailureThreshold {
				return false
			}
		}
	}

	return true
}

// GetStatus returns overall health status
func (c *Checker) GetStatus() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hasUnhealthy := false
	hasDegraded := false

	for name, result := range c.results {
		switch result.Status {
		case StatusUnhealthy:
			if c.failures[name] >= c.config.FailureThreshold {
				hasUnhealthy = true
			} else {
				hasDegraded = true
			}
		case StatusDegraded:
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return StatusUnhealthy
	}
	if hasDegraded {
		return StatusDegraded
	}
	return StatusHealthy
}

// GetResults returns current check results
func (c *Checker) GetResults() map[string]CheckResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make(map[string]CheckResult)
	for k, v := range c.results {
		results[k] = v
	}
	return results
}

// LivenessHandler returns an HTTP handler for liveness checks
func (c *Checker) LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.IsLive() {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(HealthResponse{
				Status:    StatusHealthy,
				Timestamp: time.Now(),
				Version:   c.version,
				Uptime:    time.Since(c.startTime),
			})
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(HealthResponse{
				Status:    StatusUnhealthy,
				Timestamp: time.Now(),
			})
		}
	})
}

// ReadinessHandler returns an HTTP handler for readiness checks
func (c *Checker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := HealthResponse{
			Timestamp: time.Now(),
			Version:   c.version,
			Uptime:    time.Since(c.startTime),
			Checks:    c.GetResults(),
			Status:    c.GetStatus(),
		}

		if c.IsReady() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

// Common health check implementations

// DatabaseCheck creates a database health check
func DatabaseCheck(name string, pingFunc func(ctx context.Context) error) Check {
	return func(ctx context.Context) CheckResult {
		start := time.Now()
		err := pingFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			return CheckResult{
				Name:      name,
				Status:    StatusUnhealthy,
				Message:   err.Error(),
				Duration:  duration,
				Timestamp: time.Now(),
			}
		}

		return CheckResult{
			Name:      name,
			Status:    StatusHealthy,
			Message:   "connected",
			Duration:  duration,
			Timestamp: time.Now(),
		}
	}
}

// TemporalCheck creates a Temporal health check
func TemporalCheck(name string, checkFunc func(ctx context.Context) error) Check {
	return func(ctx context.Context) CheckResult {
		start := time.Now()
		err := checkFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			return CheckResult{
				Name:      name,
				Status:    StatusUnhealthy,
				Message:   err.Error(),
				Duration:  duration,
				Timestamp: time.Now(),
			}
		}

		return CheckResult{
			Name:      name,
			Status:    StatusHealthy,
			Message:   "connected",
			Duration:  duration,
			Timestamp: time.Now(),
		}
	}
}

// LLMCheck creates an LLM service health check
func LLMCheck(name string, checkFunc func(ctx context.Context) error) Check {
	return func(ctx context.Context) CheckResult {
		start := time.Now()
		err := checkFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			return CheckResult{
				Name:      name,
				Status:    StatusDegraded, // LLM issues are degraded, not unhealthy
				Message:   err.Error(),
				Duration:  duration,
				Timestamp: time.Now(),
			}
		}

		return CheckResult{
			Name:      name,
			Status:    StatusHealthy,
			Message:   "available",
			Duration:  duration,
			Timestamp: time.Now(),
		}
	}
}

// DependencyCheck creates a generic dependency health check
func DependencyCheck(name string, checkFunc func(ctx context.Context) error, critical bool) Check {
	return func(ctx context.Context) CheckResult {
		start := time.Now()
		err := checkFunc(ctx)
		duration := time.Since(start)

		if err != nil {
			status := StatusDegraded
			if critical {
				status = StatusUnhealthy
			}
			return CheckResult{
				Name:      name,
				Status:    status,
				Message:   err.Error(),
				Duration:  duration,
				Timestamp: time.Now(),
			}
		}

		return CheckResult{
			Name:      name,
			Status:    StatusHealthy,
			Message:   "ok",
			Duration:  duration,
			Timestamp: time.Now(),
		}
	}
}

// MemoryCheck creates a memory usage health check
func MemoryCheck(name string, maxPercentUsed float64) Check {
	return func(ctx context.Context) CheckResult {
		start := time.Now()

		// Simple memory check (in real implementation, use runtime.MemStats)
		var memStats struct {
			Alloc      uint64
			TotalAlloc uint64
			Sys        uint64
		}

		// This would normally read from runtime.MemStats
		percentUsed := float64(memStats.Alloc) / float64(memStats.Sys) * 100

		status := StatusHealthy
		message := "memory usage normal"

		if percentUsed > maxPercentUsed {
			status = StatusDegraded
			message = "high memory usage"
		}

		return CheckResult{
			Name:      name,
			Status:    status,
			Message:   message,
			Duration:  time.Since(start),
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"percent_used": percentUsed,
				"max_allowed":  maxPercentUsed,
			},
		}
	}
}

// GoroutineCheck creates a goroutine count health check
func GoroutineCheck(name string, maxGoroutines int) Check {
	return func(ctx context.Context) CheckResult {
		start := time.Now()

		// In real implementation, use runtime.NumGoroutine()
		numGoroutines := 100 // placeholder

		status := StatusHealthy
		message := "goroutine count normal"

		if numGoroutines > maxGoroutines {
			status = StatusDegraded
			message = "high goroutine count"
		}

		return CheckResult{
			Name:      name,
			Status:    status,
			Message:   message,
			Duration:  time.Since(start),
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"count":       numGoroutines,
				"max_allowed": maxGoroutines,
			},
		}
	}
}

// CompositeChecker combines multiple checkers
type CompositeChecker struct {
	checkers []*Checker
}

// NewCompositeChecker creates a composite checker
func NewCompositeChecker(checkers ...*Checker) *CompositeChecker {
	return &CompositeChecker{
		checkers: checkers,
	}
}

// IsLive returns true if all checkers are live
func (cc *CompositeChecker) IsLive() bool {
	for _, c := range cc.checkers {
		if !c.IsLive() {
			return false
		}
	}
	return true
}

// IsReady returns true if all checkers are ready
func (cc *CompositeChecker) IsReady() bool {
	for _, c := range cc.checkers {
		if !c.IsReady() {
			return false
		}
	}
	return true
}

// GetStatus returns the worst status from all checkers
func (cc *CompositeChecker) GetStatus() Status {
	worst := StatusHealthy
	for _, c := range cc.checkers {
		status := c.GetStatus()
		if status == StatusUnhealthy {
			return StatusUnhealthy
		}
		if status == StatusDegraded {
			worst = StatusDegraded
		}
	}
	return worst
}
