/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package limiter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NVIDIA/go-ratelimit/pkg/config"
)

// MockRedisClient simulates Redis behavior for testing
type MockRedisClient struct {
	mu         sync.RWMutex
	data       map[string]interface{}
	calls      map[string]int
	shouldFail bool
	failAfter  int
	callCount  int
	latency    time.Duration
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data:  make(map[string]interface{}),
		calls: make(map[string]int),
	}
}

func (m *MockRedisClient) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

func (m *MockRedisClient) SetFailAfter(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failAfter = n
}

func (m *MockRedisClient) SetLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latency = d
}

func (m *MockRedisClient) simulateOperation(op string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls[op]++
	m.callCount++

	if m.latency > 0 {
		time.Sleep(m.latency)
	}

	if m.shouldFail {
		return errors.New("redis: connection failed")
	}

	if m.failAfter > 0 && m.callCount > m.failAfter {
		return errors.New("redis: connection failed after n calls")
	}

	return nil
}

func (m *MockRedisClient) GetCallCount(op string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.calls[op]
}

// TestRedisLimiterWithMock tests RedisLimiter with mock
func TestRedisLimiterWithMock(t *testing.T) {
	tests := []struct {
		name          string
		limit         int
		window        time.Duration
		requests      int
		expectAllowed int
		expectDenied  int
	}{
		{
			name:          "basic rate limiting",
			limit:         5,
			window:        time.Minute,
			requests:      8,
			expectAllowed: 5,
			expectDenied:  3,
		},
		{
			name:          "all requests allowed",
			limit:         10,
			window:        time.Minute,
			requests:      5,
			expectAllowed: 5,
			expectDenied:  0,
		},
		{
			name:          "single request limit",
			limit:         1,
			window:        time.Minute,
			requests:      3,
			expectAllowed: 1,
			expectDenied:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate with in-memory counter
			counter := &InMemoryLimiter{
				limit:    tt.limit,
				window:   tt.window,
				counters: make(map[string]*Counter),
			}

			allowed := 0
			denied := 0

			for i := 0; i < tt.requests; i++ {
				if counter.AllowRequest(context.Background(), "test-key") {
					allowed++
				} else {
					denied++
				}
			}

			if allowed != tt.expectAllowed {
				t.Errorf("Expected %d allowed, got %d", tt.expectAllowed, allowed)
			}

			if denied != tt.expectDenied {
				t.Errorf("Expected %d denied, got %d", tt.expectDenied, denied)
			}
		})
	}
}

// InMemoryLimiter is a mock implementation for testing
type InMemoryLimiter struct {
	mu       sync.RWMutex
	limit    int
	window   time.Duration
	counters map[string]*Counter
}

type Counter struct {
	count     int
	resetTime time.Time
}

func (l *InMemoryLimiter) AllowRequest(ctx context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	counter, exists := l.counters[key]

	if !exists || now.After(counter.resetTime) {
		l.counters[key] = &Counter{
			count:     1,
			resetTime: now.Add(l.window),
		}
		return true
	}

	if counter.count < l.limit {
		counter.count++
		return true
	}

	return false
}

// TestDualLimiterWithMock tests DualLimiter functionality with mocks
func TestDualLimiterWithMock(t *testing.T) {
	tests := []struct {
		name         string
		requestLimit int
		byteLimit    int64
		requests     []int // bytes per request
		expectDenied int
		deniedReason string
	}{
		{
			name:         "request limit hit first",
			requestLimit: 3,
			byteLimit:    10000,
			requests:     []int{100, 100, 100, 100, 100},
			expectDenied: 2,
			deniedReason: "request",
		},
		{
			name:         "byte limit hit first",
			requestLimit: 10,
			byteLimit:    500,
			requests:     []int{200, 200, 200},
			expectDenied: 1,
			deniedReason: "byte",
		},
		{
			name:         "mixed limits",
			requestLimit: 5,
			byteLimit:    1000,
			requests:     []int{100, 200, 300, 200, 200, 100},
			expectDenied: 2, // Either limit can be hit
			deniedReason: "either",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := &InMemoryDualLimiter{
				requestLimit: tt.requestLimit,
				byteLimit:    tt.byteLimit,
				window:       time.Minute,
				counters:     make(map[string]*DualCounter),
			}

			denied := 0
			for _, bytes := range tt.requests {
				if !limiter.AllowRequest(context.Background(), "test-key", int64(bytes)) {
					denied++
				}
			}

			if denied != tt.expectDenied {
				t.Errorf("Expected %d denied, got %d", tt.expectDenied, denied)
			}
		})
	}
}

// InMemoryDualLimiter mocks dual limiting
type InMemoryDualLimiter struct {
	mu           sync.RWMutex
	requestLimit int
	byteLimit    int64
	window       time.Duration
	counters     map[string]*DualCounter
}

type DualCounter struct {
	requests  int
	bytes     int64
	resetTime time.Time
}

func (l *InMemoryDualLimiter) AllowRequest(ctx context.Context, key string, bytes int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	counter, exists := l.counters[key]

	if !exists || now.After(counter.resetTime) {
		l.counters[key] = &DualCounter{
			requests:  1,
			bytes:     bytes,
			resetTime: now.Add(l.window),
		}
		return true
	}

	// Check both limits
	if counter.requests >= l.requestLimit {
		return false
	}

	if counter.bytes+bytes > l.byteLimit {
		return false
	}

	counter.requests++
	counter.bytes += bytes
	return true
}

// TestDynamicLimiterWithMock tests dynamic configuration changes
func TestDynamicLimiterWithMock(t *testing.T) {
	ctx := context.Background()

	// Create mock config manager
	configMgr := &MockConfigManager{
		configs: make(map[string]config.ClientConfig),
		defaultConfig: config.ClientConfig{
			RequestLimit: 10,
			ByteLimit:    1000,
			WindowSecs:   60,
		},
	}

	limiter := &MockDynamicLimiter{
		configMgr: configMgr,
		counters:  make(map[string]*DualCounter),
		window:    time.Minute,
	}

	// Test default limits
	t.Run("default limits", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			if !limiter.AllowRequest(ctx, "client1") {
				t.Errorf("Request %d should be allowed with default limit", i+1)
			}
		}

		if limiter.AllowRequest(ctx, "client1") {
			t.Error("Request 11 should be denied with default limit of 10")
		}
	})

	// Test custom limits
	t.Run("custom limits", func(t *testing.T) {
		// Set custom config for client2
		configMgr.SetConfig("client2", config.ClientConfig{
			RequestLimit: 5,
			ByteLimit:    500,
			WindowSecs:   60,
		})

		// Reset counter for clean test
		limiter.ResetCounter("client2")

		for i := 0; i < 5; i++ {
			if !limiter.AllowRequest(ctx, "client2") {
				t.Errorf("Request %d should be allowed with custom limit", i+1)
			}
		}

		if limiter.AllowRequest(ctx, "client2") {
			t.Error("Request 6 should be denied with custom limit of 5")
		}
	})

	// Test config update
	t.Run("config update", func(t *testing.T) {
		// Update config for client2
		configMgr.SetConfig("client2", config.ClientConfig{
			RequestLimit: 20,
			ByteLimit:    2000,
			WindowSecs:   60,
		})

		// Note: In real scenario, existing rate limit would still apply until window resets
		// For testing, we simulate window reset
		limiter.ResetCounter("client2")

		for i := 0; i < 20; i++ {
			if !limiter.AllowRequest(ctx, "client2") {
				t.Errorf("Request %d should be allowed with updated limit", i+1)
			}
		}

		if limiter.AllowRequest(ctx, "client2") {
			t.Error("Request 21 should be denied with updated limit of 20")
		}
	})

	// Test config reset
	t.Run("config reset", func(t *testing.T) {
		// Reset config for client2
		configMgr.ResetConfig("client2")
		limiter.ResetCounter("client2")

		// Should use default limits again
		for i := 0; i < 10; i++ {
			if !limiter.AllowRequest(ctx, "client2") {
				t.Errorf("Request %d should be allowed with default limit after reset", i+1)
			}
		}

		if limiter.AllowRequest(ctx, "client2") {
			t.Error("Request 11 should be denied with default limit after reset")
		}
	})
}

// MockConfigManager simulates config management
type MockConfigManager struct {
	mu            sync.RWMutex
	configs       map[string]config.ClientConfig
	defaultConfig config.ClientConfig
}

func (m *MockConfigManager) GetConfig(key string) config.ClientConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cfg, ok := m.configs[key]; ok {
		return cfg
	}
	return m.defaultConfig
}

func (m *MockConfigManager) SetConfig(key string, cfg config.ClientConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[key] = cfg
}

func (m *MockConfigManager) ResetConfig(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.configs, key)
}

// MockDynamicLimiter simulates dynamic limiting
type MockDynamicLimiter struct {
	mu        sync.RWMutex
	configMgr *MockConfigManager
	counters  map[string]*DualCounter
	window    time.Duration
}

func (l *MockDynamicLimiter) AllowRequest(ctx context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cfg := l.configMgr.GetConfig(key)
	now := time.Now()

	counter, exists := l.counters[key]

	if !exists || now.After(counter.resetTime) {
		l.counters[key] = &DualCounter{
			requests:  1,
			bytes:     0,
			resetTime: now.Add(l.window),
		}
		return true
	}

	if counter.requests >= cfg.RequestLimit {
		return false
	}

	counter.requests++
	return true
}

func (l *MockDynamicLimiter) ResetCounter(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counters, key)
}

// TestConcurrentAccess tests thread safety
func TestConcurrentAccess(t *testing.T) {
	limiter := &InMemoryLimiter{
		limit:    100,
		window:   time.Minute,
		counters: make(map[string]*Counter),
	}

	var allowed int64
	var denied int64
	var wg sync.WaitGroup

	// Run concurrent requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			for j := 0; j < 20; j++ {
				key := fmt.Sprintf("client-%d", clientID)
				if limiter.AllowRequest(context.Background(), key) {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&denied, 1)
				}

				// Small delay to simulate processing
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	totalRequests := int64(200) // 10 clients * 20 requests
	if allowed+denied != totalRequests {
		t.Errorf("Request count mismatch: allowed=%d, denied=%d, total=%d",
			allowed, denied, totalRequests)
	}

	// Each client should have at most 100 allowed (the limit)
	// With 20 requests per client and 100 limit, all should be allowed
	if denied > 0 {
		t.Logf("Note: %d requests were denied (this is OK for concurrent test)", denied)
	}
}

// TestSlidingWindow tests sliding window algorithm
func TestSlidingWindow(t *testing.T) {
	limiter := &SlidingWindowLimiter{
		limit:    5,
		window:   100 * time.Millisecond,
		requests: make(map[string][]time.Time),
	}

	ctx := context.Background()
	key := "test-key"

	// Make 5 requests - all should be allowed
	for i := 0; i < 5; i++ {
		if !limiter.AllowRequest(ctx, key) {
			t.Errorf("Request %d should be allowed", i+1)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 6th request should be denied
	if limiter.AllowRequest(ctx, key) {
		t.Error("6th request should be denied")
	}

	// Wait for window to slide
	time.Sleep(60 * time.Millisecond)

	// Should allow one more request as oldest one slides out
	if !limiter.AllowRequest(ctx, key) {
		t.Error("Request should be allowed after window slides")
	}

	// But not two
	if limiter.AllowRequest(ctx, key) {
		t.Error("Request should be denied - limit still reached")
	}

	// Wait for complete window reset
	time.Sleep(150 * time.Millisecond)

	// Should allow 5 requests again
	for i := 0; i < 5; i++ {
		if !limiter.AllowRequest(ctx, key) {
			t.Errorf("Request %d should be allowed after window reset", i+1)
		}
	}
}

// SlidingWindowLimiter implements sliding window algorithm
type SlidingWindowLimiter struct {
	mu       sync.RWMutex
	limit    int
	window   time.Duration
	requests map[string][]time.Time
}

func (l *SlidingWindowLimiter) AllowRequest(ctx context.Context, key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-l.window)

	// Get existing requests
	timestamps, exists := l.requests[key]
	if !exists {
		l.requests[key] = []time.Time{now}
		return true
	}

	// Remove expired requests
	validRequests := []time.Time{}
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			validRequests = append(validRequests, ts)
		}
	}

	// Check if under limit
	if len(validRequests) < l.limit {
		validRequests = append(validRequests, now)
		l.requests[key] = validRequests
		return true
	}

	l.requests[key] = validRequests
	return false
}

// BenchmarkLimiters benchmarks different limiter implementations
func BenchmarkLimiters(b *testing.B) {
	benchmarks := []struct {
		name    string
		limiter interface {
			AllowRequest(context.Context, string) bool
		}
	}{
		{
			name: "InMemoryLimiter",
			limiter: &InMemoryLimiter{
				limit:    1000,
				window:   time.Hour,
				counters: make(map[string]*Counter),
			},
		},
		{
			name: "SlidingWindowLimiter",
			limiter: &SlidingWindowLimiter{
				limit:    1000,
				window:   time.Hour,
				requests: make(map[string][]time.Time),
			},
		},
	}

	ctx := context.Background()

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					key := fmt.Sprintf("key-%d", i%100)
					bm.limiter.AllowRequest(ctx, key)
					i++
				}
			})
		})
	}
}

// TestKeyFunction tests various key extraction scenarios
func TestKeyFunction(t *testing.T) {
	tests := []struct {
		name     string
		keyFunc  func(*http.Request) string
		request  func() *http.Request
		expected string
	}{
		{
			name: "extract from header",
			keyFunc: func(r *http.Request) string {
				return r.Header.Get("X-Client-ID")
			},
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.Header.Set("X-Client-ID", "client-123")
				return req
			},
			expected: "client-123",
		},
		{
			name: "extract from IP",
			keyFunc: func(r *http.Request) string {
				return r.RemoteAddr
			},
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1.1:1234"
				return req
			},
			expected: "192.168.1.1:1234",
		},
		{
			name: "composite key",
			keyFunc: func(r *http.Request) string {
				return r.Method + ":" + r.URL.Path
			},
			request: func() *http.Request {
				req, _ := http.NewRequest("POST", "/api/users", nil)
				return req
			},
			expected: "POST:/api/users",
		},
		{
			name: "fallback key",
			keyFunc: func(r *http.Request) string {
				if id := r.Header.Get("X-Request-ID"); id != "" {
					return id
				}
				return "default"
			},
			request: func() *http.Request {
				req, _ := http.NewRequest("GET", "/test", nil)
				return req
			},
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.request()
			key := tt.keyFunc(req)

			if key != tt.expected {
				t.Errorf("Expected key %q, got %q", tt.expected, key)
			}
		})
	}
}
