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
package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/config"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
)

// setupMockRedis sets up a mock Redis client for testing
func setupMockRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // This won't be used in tests
	})
}

// TestResetClientConfig tests the ResetClientConfig functionality
func TestResetClientConfig(t *testing.T) {
	// Set up mock Redis client
	rdb := setupMockRedis()

	// Create a dynamic limiter with test configuration
	dynamicLimiter := limiter.NewDynamicLimiter(
		rdb,
		"test-prefix",
		10,      // Default request limit
		1048576, // Default byte limit (1MB)
		60,      // Default window seconds
		func(r *http.Request) string { return "test-client" },
	)

	// Create the service with the limiter
	cfg := &config.Config{}
	svc := NewBasicService(cfg, dynamicLimiter)

	// Test the ResetClientConfig method
	req := &ratelimit_pb.ResetClientConfigRequest{
		ClientId: "test-client-123",
	}

	// Mock the ResetClientConfig method in the limiter
	// In a real test, you would use a mock library to mock the limiter
	// For this example, we'll just test that the service doesn't panic

	// Call the service method
	resp, err := svc.ResetClientConfig(context.Background(), req)

	// Assert that there was no error
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Status)
}

// TestGetClientConfig tests the GetClientConfig functionality
func TestGetClientConfig(t *testing.T) {
	// Set up mock Redis client
	rdb := setupMockRedis()

	// Create a dynamic limiter with test configuration
	dynamicLimiter := limiter.NewDynamicLimiter(
		rdb,
		"test-prefix",
		10,      // Default request limit
		1048576, // Default byte limit (1MB)
		60,      // Default window seconds
		func(r *http.Request) string { return "test-client" },
	)

	// Create the service with the limiter
	cfg := &config.Config{}
	svc := NewBasicService(cfg, dynamicLimiter)

	// Test the GetClientConfig method
	req := &ratelimit_pb.GetClientConfigRequest{
		ClientId: "test-client-123",
	}

	// Call the service method
	resp, err := svc.GetClientConfig(context.Background(), req)

	// Assert that there was no error
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Config)
	assert.Equal(t, int32(10), resp.Config.RequestLimit)
	assert.Equal(t, int64(1048576), resp.Config.ByteLimit)
	assert.Equal(t, int32(60), resp.Config.WindowSecs)
}

// TestUpdateClientConfig tests the UpdateClientConfig functionality
func TestUpdateClientConfig(t *testing.T) {
	// Set up mock Redis client
	rdb := setupMockRedis()

	// Create a dynamic limiter with test configuration
	dynamicLimiter := limiter.NewDynamicLimiter(
		rdb,
		"test-prefix",
		10,      // Default request limit
		1048576, // Default byte limit (1MB)
		60,      // Default window seconds
		func(r *http.Request) string { return "test-client" },
	)

	// Create the service with the limiter
	cfg := &config.Config{}
	svc := NewBasicService(cfg, dynamicLimiter)

	// Test the UpdateClientConfig method
	req := &ratelimit_pb.UpdateClientConfigRequest{
		ClientId: "test-client-123",
		Config: &ratelimit_pb.ClientConfig{
			RequestLimit: 20,
			ByteLimit:    2097152, // 2MB
			WindowSecs:   120,     // 2 minutes
		},
	}

	// Call the service method
	resp, err := svc.UpdateClientConfig(context.Background(), req)

	// Assert that there was no error
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "ok", resp.Status)

	// Verify the update by getting the config
	getReq := &ratelimit_pb.GetClientConfigRequest{
		ClientId: "test-client-123",
	}
	getResp, err := svc.GetClientConfig(context.Background(), getReq)
	require.NoError(t, err)
	assert.NotNil(t, getResp)
	assert.NotNil(t, getResp.Config)
	assert.Equal(t, int32(20), getResp.Config.RequestLimit)
	assert.Equal(t, int64(2097152), getResp.Config.ByteLimit)
	assert.Equal(t, int32(120), getResp.Config.WindowSecs)
}
