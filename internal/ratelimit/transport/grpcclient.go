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
package transport

import (
	"context"
	"time"

	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/endpoint"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/service"

	"github.com/go-kit/kit/circuitbreaker"
	kitendpoint "github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/ratelimit"
	grpctransport "github.com/go-kit/kit/transport/grpc"
	"github.com/pkg/errors"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

// NewGRPCClient returns an ratelimit_pbService backed by a gRPC server at the other end
// NewGRPCClient returns an kdo_pbService backed by a gRPC server at the other end
// of the conn. The caller is responsible for constructing the conn, and
// eventually closing the underlying transport. We bake-in certain middlewares,
// implementing the client library pattern.
func NewGRPCClient(conn *grpc.ClientConn, logger log.Logger) service.Service {
	// We construct a single ratelimiter middleware, to limit the total outgoing
	// QPS from this client to all methods on the remote instance. We also
	// construct per-endpoint circuitbreaker middlewares to demonstrate how
	// that's done, although they could easily be combined into a single breaker
	// for the entire remote instance, too.
	limiter := ratelimit.NewErroringLimiter(rate.NewLimiter(rate.Every(time.Second), 100))

	// global client middlewares
	var options []grpctransport.ClientOption

	var getClientConfigEndpoint kitendpoint.Endpoint
	{
		getClientConfigEndpoint = grpctransport.NewClient(
			conn,
			"gitlab-master.nvidia.com.kratos-dataverse-observability.go-ratelimit.api.nvidia.v1.AdminService",
			"GetClientConfig",
			encodeGRPCGetClientConfigRequest,
			decodeGRPCGetClientConfigResponse,
			ratelimit_pb.ClientConfigResponse{},
			options...,
		).Endpoint()
		getClientConfigEndpoint = limiter(getClientConfigEndpoint)
		getClientConfigEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "GetClientConfig",
			Timeout: 30 * time.Second,
		}))(getClientConfigEndpoint)
	}

	var updateClientConfigEndpoint kitendpoint.Endpoint
	{
		updateClientConfigEndpoint = grpctransport.NewClient(
			conn,
			"gitlab-master.nvidia.com.kratos-dataverse-observability.go-ratelimit.api.nvidia.v1.AdminService",
			"UpdateClientConfig",
			encodeGRPCUpdateClientConfigRequest,
			decodeGRPCUpdateClientConfigResponse,
			ratelimit_pb.StatusResponse{},
			options...,
		).Endpoint()
		updateClientConfigEndpoint = limiter(updateClientConfigEndpoint)
		updateClientConfigEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "UpdateClientConfig",
			Timeout: 30 * time.Second,
		}))(updateClientConfigEndpoint)
	}

	var resetClientConfigEndpoint kitendpoint.Endpoint
	{
		resetClientConfigEndpoint = grpctransport.NewClient(
			conn,
			"gitlab-master.nvidia.com.kratos-dataverse-observability.go-ratelimit.api.nvidia.v1.AdminService",
			"ResetClientConfig",
			encodeGRPCResetClientConfigRequest,
			decodeGRPCResetClientConfigResponse,
			ratelimit_pb.StatusResponse{},
			options...,
		).Endpoint()
		resetClientConfigEndpoint = limiter(resetClientConfigEndpoint)
		resetClientConfigEndpoint = circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "ResetClientConfig",
			Timeout: 30 * time.Second,
		}))(resetClientConfigEndpoint)
	}

	// Returning the endpoint.Set as a service.Service relies on the
	// endpoint.Set implementing the Service methods. That's just a simple bit
	// of glue code.
	return endpoint.Set{
		GetClientConfigEndpoint:    getClientConfigEndpoint,
		UpdateClientConfigEndpoint: updateClientConfigEndpoint,
		ResetClientConfigEndpoint:  resetClientConfigEndpoint,
	}
}

// encodeGRPCGetClientConfigRequest is a transport/grpc.EncodeRequestFunc that converts a
// user-domain GetClientConfig request to a gRPC GetClientConfig request. Primarily useful in a client.
func encodeGRPCGetClientConfigRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(endpoint.GetClientConfigRequest)
	return req.Request, nil
}

// decodeGRPCGetClientConfigResponse is a transport/grpc.DecodeResponseFunc that converts a
// gRPC GetClientConfig reply to a user-domain GetClientConfig response. Primarily useful in a client.
func decodeGRPCGetClientConfigResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*ratelimit_pb.ClientConfigResponse)
	return endpoint.GetClientConfigResponse{Response: reply, Err: nil}, nil
}

// encodeGRPCUpdateClientConfigRequest is a transport/grpc.EncodeRequestFunc that converts a
// user-domain UpdateClientConfig request to a gRPC UpdateClientConfig request. Primarily useful in a client.
func encodeGRPCUpdateClientConfigRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(endpoint.UpdateClientConfigRequest)
	return req.Request, nil
}

// decodeGRPCUpdateClientConfigResponse is a transport/grpc.DecodeResponseFunc that converts a
// gRPC UpdateClientConfig reply to a user-domain UpdateClientConfig response. Primarily useful in a client.
func decodeGRPCUpdateClientConfigResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*ratelimit_pb.StatusResponse)
	return endpoint.UpdateClientConfigResponse{Response: reply, Err: nil}, nil
}

// encodeGRPCResetClientConfigRequest is a transport/grpc.EncodeRequestFunc that converts a
// user-domain ResetClientConfig request to a gRPC ResetClientConfig request. Primarily useful in a client.
func encodeGRPCResetClientConfigRequest(_ context.Context, request interface{}) (interface{}, error) {
	req := request.(endpoint.ResetClientConfigRequest)
	return req.Request, nil
}

// decodeGRPCResetClientConfigResponse is a transport/grpc.DecodeResponseFunc that converts a
// gRPC ResetClientConfig reply to a user-domain ResetClientConfig response. Primarily useful in a client.
func decodeGRPCResetClientConfigResponse(_ context.Context, grpcReply interface{}) (interface{}, error) {
	reply := grpcReply.(*ratelimit_pb.StatusResponse)
	return endpoint.ResetClientConfigResponse{Response: reply, Err: nil}, nil
}

//ons are required to translate Go error types to
// and from strings, which is the type we use in our IDLs to represent errors.
// There is special casing to treat empty strings as nil errors.

func str2err(s string) error {
	if s == "" {
		return nil
	}
	return errors.New(s)
}

func err2str(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
