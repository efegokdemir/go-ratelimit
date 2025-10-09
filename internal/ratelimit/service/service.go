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
	"fmt"

	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/config"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
	"go.opentelemetry.io/otel"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/log"
	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
)

// Service describes a service that adds things together.
type Service interface {
	// Admin API endpoints
	GetClientConfig(ctx context.Context, req *ratelimit_pb.GetClientConfigRequest) (*ratelimit_pb.ClientConfigResponse, error)
	UpdateClientConfig(ctx context.Context, req *ratelimit_pb.UpdateClientConfigRequest) (*ratelimit_pb.StatusResponse, error)
	ResetClientConfig(ctx context.Context, req *ratelimit_pb.ResetClientConfigRequest) (*ratelimit_pb.StatusResponse, error)
}

// New returns a basic Service with all of the expected middlewares wired in.
func New(logger log.Logger, helloCtr metrics.Counter, cfg *config.Config, dynamicLimiter *limiter.DynamicLimiter) Service {
	var svc Service
	{
		svc = NewBasicService(cfg, dynamicLimiter)
		svc = LoggingMiddleware(logger)(svc)
		svc = InstrumentingMiddleware(helloCtr)(svc)
	}
	return svc
}

// NewBasicService returns a naïve, stateless implementation of Service.
func NewBasicService(cfg *config.Config, dynamicLimiter *limiter.DynamicLimiter) Service {
	return basicService{
		config:  cfg,
		limiter: dynamicLimiter,
	}
}

type basicService struct {
	config  *config.Config
	limiter *limiter.DynamicLimiter
}

func (s basicService) GetClientConfig(ctx context.Context, req *ratelimit_pb.GetClientConfigRequest) (*ratelimit_pb.ClientConfigResponse, error) {
	_, span := otel.Tracer("ratelimit-admin").Start(ctx, "GetClientConfig")
	defer span.End()

	// Check if limiter is available
	if s.limiter == nil {
		return nil, fmt.Errorf("rate limiter not initialized")
	}

	// Get client configuration using the limiter
	clientID := req.ClientId
	cfg, err := s.limiter.GetClientConfig(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("error getting client config: %w", err)
	}

	// Convert to protobuf response
	response := &ratelimit_pb.ClientConfigResponse{
		Config: &ratelimit_pb.ClientConfig{
			RequestLimit: int32(cfg.RequestLimit),
			ByteLimit:    cfg.ByteLimit,
			WindowSecs:   int32(cfg.WindowSecs),
		},
	}

	return response, nil
}

func (s basicService) UpdateClientConfig(ctx context.Context, req *ratelimit_pb.UpdateClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	_, span := otel.Tracer("ratelimit-admin").Start(ctx, "UpdateClientConfig")
	defer span.End()

	// Check if limiter is available
	if s.limiter == nil {
		return nil, fmt.Errorf("rate limiter not initialized")
	}

	// Convert from protobuf request to config
	clientID := req.ClientId
	cfg := config.ClientConfig{
		RequestLimit: int(req.Config.RequestLimit),
		ByteLimit:    req.Config.ByteLimit,
		WindowSecs:   int(req.Config.WindowSecs),
	}

	// Validate the configuration
	if cfg.RequestLimit <= 0 {
		return nil, fmt.Errorf("request limit must be positive")
	}
	if cfg.ByteLimit <= 0 {
		return nil, fmt.Errorf("byte limit must be positive")
	}
	if cfg.WindowSecs <= 0 {
		return nil, fmt.Errorf("window seconds must be positive")
	}

	// Update client configuration using the limiter
	err := s.limiter.SetClientConfig(ctx, clientID, cfg)
	if err != nil {
		return nil, fmt.Errorf("error updating client config: %w", err)
	}

	// Return success response
	response := &ratelimit_pb.StatusResponse{
		Status: "ok",
	}

	return response, nil
}

func (s basicService) ResetClientConfig(ctx context.Context, req *ratelimit_pb.ResetClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	_, span := otel.Tracer("ratelimit-admin").Start(ctx, "ResetClientConfig")
	defer span.End()

	// Check if limiter is available
	if s.limiter == nil {
		return nil, fmt.Errorf("rate limiter not initialized")
	}

	// Reset client configuration using the limiter
	clientID := req.ClientId
	err := s.limiter.ResetClientConfig(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("error resetting client config: %w", err)
	}

	// Return success response
	response := &ratelimit_pb.StatusResponse{
		Status: "ok",
	}

	return response, nil
}
