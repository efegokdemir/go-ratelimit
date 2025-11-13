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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/config"
)

// DynamicLimiter implements a rate limiter with per-client configurations
type DynamicLimiter struct {
	rdb        redis.Cmdable
	prefix     string
	cfgManager *config.ConfigManager
	lua        *redis.Script
	keyBuilder func(*http.Request) string
}

// NewDynamicLimiter creates a rate limiter with dynamic per-client configurations
func NewDynamicLimiter(
	rdb redis.Cmdable,
	prefix string,
	defaultRequestLimit int,
	defaultByteLimit int64,
	defaultWindow time.Duration,
	keyBuilder func(*http.Request) string,
) *DynamicLimiter {
	// Create default config
	defaultCfg := config.ClientConfig{
		RequestLimit: defaultRequestLimit,
		ByteLimit:    defaultByteLimit,
		WindowSecs:   int(defaultWindow.Seconds()),
	}

	// Create config manager
	cfgManager := config.NewConfigManager(rdb, prefix, defaultCfg)

	return &DynamicLimiter{
		rdb:        rdb,
		prefix:     prefix,
		cfgManager: cfgManager,
		lua:        redis.NewScript(dualSlidingLua), // Reuse the same Lua script
		keyBuilder: keyBuilder,
	}
}

func (dl *DynamicLimiter) requestKey(k string) string { return dl.prefix + ":req:" + k }
func (dl *DynamicLimiter) byteKey(k string) string    { return dl.prefix + ":byte:" + k }

// GetConfigManager returns the config manager for setting client-specific configs
func (dl *DynamicLimiter) GetConfigManager() *config.ConfigManager {
	return dl.cfgManager
}

// extractClientID extracts the client identifier from the key
func extractClientID(key string) string {
	// Extract API key from the composite key
	if strings.HasPrefix(key, "apikey:") {
		parts := strings.Split(key, "|")
		if len(parts) > 0 {
			return strings.TrimPrefix(parts[0], "apikey:")
		}
	}
	return key
}

// AllowRequest checks if a new request is allowed based on client-specific config
func (dl *DynamicLimiter) AllowRequest(ctx context.Context, r *http.Request) (bool, int64, int64, error) {
	// Get the key for this request
	k := dl.keyBuilder(r)

	// Extract client ID for config lookup
	clientID := extractClientID(k)

	// Get client-specific config
	cfg, err := dl.cfgManager.GetConfig(ctx, clientID)
	if err != nil {
		// Log the error but continue with default config
		// This is a fail-open approach for config errors
	}

	// Execute Lua script with client-specific limits
	windowMs := int64(cfg.WindowSecs * 1000)
	res, err := dl.lua.Run(ctx, dl.rdb,
		[]string{dl.requestKey(k), dl.byteKey(k)},
		windowMs, cfg.RequestLimit, cfg.ByteLimit, 0).Result()
	if err != nil {
		return false, 0, 0, err
	}

	arr := res.([]interface{})
	allowed := arr[0].(int64) == 1
	remaining := arr[1].(int64)
	reset := arr[2].(int64)
	return allowed, remaining, reset, nil
}

// GetLastKey returns the last key used by this limiter
func (dl *DynamicLimiter) GetLastKey(r *http.Request) string {
	return dl.keyBuilder(r)
}

// RecordBytes records the bytes used for a request with client-specific config
func (dl *DynamicLimiter) RecordBytes(ctx context.Context, r *http.Request, bytes int64) (bool, int64, int64, error) {
	// Get the key for this request
	k := dl.keyBuilder(r)

	// Extract client ID for config lookup
	clientID := extractClientID(k)

	// Get client-specific config
	cfg, err := dl.cfgManager.GetConfig(ctx, clientID)
	if err != nil {
		// Log the error but continue with default config
	}

	// Execute Lua script with client-specific limits
	windowMs := int64(cfg.WindowSecs * 1000)
	res, err := dl.lua.Run(ctx, dl.rdb,
		[]string{dl.requestKey(k), dl.byteKey(k)},
		windowMs, cfg.RequestLimit, cfg.ByteLimit, bytes).Result()
	if err != nil {
		return false, 0, 0, err
	}

	arr := res.([]interface{})
	allowed := arr[0].(int64) == 1
	remaining := arr[1].(int64)
	reset := arr[2].(int64)
	return allowed, remaining, reset, nil
}

// GetClientConfig returns the current config for a client
func (dl *DynamicLimiter) GetClientConfig(ctx context.Context, clientID string) (config.ClientConfig, error) {
	return dl.cfgManager.GetConfig(ctx, clientID)
}

// SetClientConfig sets a custom config for a client
func (dl *DynamicLimiter) SetClientConfig(ctx context.Context, clientID string, cfg config.ClientConfig) error {
	return dl.cfgManager.SetConfig(ctx, clientID, cfg)
}

// ResetClientConfig removes custom config for a client, reverting to defaults
func (dl *DynamicLimiter) ResetClientConfig(ctx context.Context, clientID string) error {
	return dl.cfgManager.DeleteConfig(ctx, clientID)
}

// GetCurrentUsage returns the current byte usage for a client without recording new bytes
// OPTIMIZED VERSION: Uses hash-based total instead of iterating through all entries
func (dl *DynamicLimiter) GetCurrentUsage(ctx context.Context, r *http.Request) (int64, error) {
	k := dl.keyBuilder(r)
	byteKey := dl.byteKey(k)
	byteTotalKey := byteKey + ":total"
	clientID := extractClientID(k)

	// Get client config to get window duration
	cfg, _ := dl.cfgManager.GetConfig(ctx, clientID)

	// Get current time
	now := time.Now().UnixMilli()
	windowMs := int64(cfg.WindowSecs * 1000)
	minTime := now - windowMs

	// Get entries that will be pruned
	prunedEntries, err := dl.rdb.ZRangeByScore(ctx, byteKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", minTime),
	}).Result()
	if err != nil {
		return 0, err
	}

	// Subtract bytes from pruned entries before removing them
	var bytesToSubtract int64
	for _, entry := range prunedEntries {
		parts := strings.Split(entry, "-")
		if len(parts) > 0 {
			if bytes, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				bytesToSubtract += bytes
			}
		}
	}

	// Update the total if we have bytes to subtract
	if bytesToSubtract > 0 {
		dl.rdb.HIncrBy(ctx, byteTotalKey, "total", -bytesToSubtract)
	}

	// Remove expired entries
	dl.rdb.ZRemRangeByScore(ctx, byteKey, "-inf", fmt.Sprintf("%d", minTime))

	// Get total from hash (O(1) operation)
	totalStr, err := dl.rdb.HGet(ctx, byteTotalKey, "total").Result()
	if err != nil && err != redis.Nil {
		return 0, err
	}

	totalBytes := int64(0)
	if totalStr != "" {
		totalBytes, _ = strconv.ParseInt(totalStr, 10, 64)
	}

	// Ensure non-negative
	if totalBytes < 0 {
		totalBytes = 0
		// Reset the hash to 0
		dl.rdb.HSet(ctx, byteTotalKey, "total", 0)
	}

	return totalBytes, nil
}
