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

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
)

// Middleware describes a service (as opposed to endpoint) middleware.
type Middleware func(Service) Service

// LoggingMiddleware takes a logger as a dependency
// and returns a service Middleware.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return loggingMiddleware{logger, next}
	}
}

type loggingMiddleware struct {
	logger log.Logger
	next   Service
}

func (mw loggingMiddleware) GetClientConfig(ctx context.Context, req *ratelimit_pb.GetClientConfigRequest) (resp *ratelimit_pb.ClientConfigResponse, err error) {
	defer func() {
		mw.logger.Log("method", "GetClientConfig", "client_id", req.ClientId, "err", err)
	}()
	return mw.next.GetClientConfig(ctx, req)
}

func (mw loggingMiddleware) UpdateClientConfig(ctx context.Context, req *ratelimit_pb.UpdateClientConfigRequest) (resp *ratelimit_pb.StatusResponse, err error) {
	defer func() {
		mw.logger.Log("method", "UpdateClientConfig", "client_id", req.ClientId, "err", err)
	}()
	return mw.next.UpdateClientConfig(ctx, req)
}

func (mw loggingMiddleware) ResetClientConfig(ctx context.Context, req *ratelimit_pb.ResetClientConfigRequest) (resp *ratelimit_pb.StatusResponse, err error) {
	defer func() {
		mw.logger.Log("method", "ResetClientConfig", "client_id", req.ClientId, "err", err)
	}()
	return mw.next.ResetClientConfig(ctx, req)
}

// InstrumentingMiddleware returns a service middleware that instruments
// the number of integers summed and characters concatenated over the lifetime of
// the service.
func InstrumentingMiddleware(helloCtr metrics.Counter) Middleware {
	return func(next Service) Service {
		return instrumentingMiddleware{
			helloCtr: helloCtr,
			next:     next,
		}
	}
}

type instrumentingMiddleware struct {
	helloCtr metrics.Counter
	next     Service
}

func (mw instrumentingMiddleware) GetClientConfig(ctx context.Context, req *ratelimit_pb.GetClientConfigRequest) (*ratelimit_pb.ClientConfigResponse, error) {
	return mw.next.GetClientConfig(ctx, req)
}

func (mw instrumentingMiddleware) UpdateClientConfig(ctx context.Context, req *ratelimit_pb.UpdateClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	return mw.next.UpdateClientConfig(ctx, req)
}

func (mw instrumentingMiddleware) ResetClientConfig(ctx context.Context, req *ratelimit_pb.ResetClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	return mw.next.ResetClientConfig(ctx, req)
}
