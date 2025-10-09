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

	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/endpoint"

	"github.com/go-kit/kit/auth/jwt"
	"github.com/go-kit/kit/log"
	kittransport "github.com/go-kit/kit/transport"
	grpctransport "github.com/go-kit/kit/transport/grpc"
)

type grpcServer struct {
	ratelimit_pb.UnimplementedAdminServiceServer
	getClientConfig    grpctransport.Handler
	updateClientConfig grpctransport.Handler
	resetClientConfig  grpctransport.Handler
}

// NewGRPCServer makes a set of endpoints available as a gRPC AddServer.
func NewGRPCServer(endpoints endpoint.Set, logger log.Logger) ratelimit_pb.AdminServiceServer {
	options := []grpctransport.ServerOption{
		grpctransport.ServerErrorHandler(kittransport.NewLogErrorHandler(logger)),
		grpctransport.ServerBefore(jwt.GRPCToContext()),
	}

	return &grpcServer{
		getClientConfig: grpctransport.NewServer(
			endpoints.GetClientConfigEndpoint,
			decodeGRPCGetClientConfigRequest,
			encodeGRPCGetClientConfigResponse,
			options...,
		),
		updateClientConfig: grpctransport.NewServer(
			endpoints.UpdateClientConfigEndpoint,
			decodeGRPCUpdateClientConfigRequest,
			encodeGRPCUpdateClientConfigResponse,
			options...,
		),
		resetClientConfig: grpctransport.NewServer(
			endpoints.ResetClientConfigEndpoint,
			decodeGRPCResetClientConfigRequest,
			encodeGRPCResetClientConfigResponse,
			options...,
		),
	}
}

func (s *grpcServer) GetClientConfig(ctx context.Context, req *ratelimit_pb.GetClientConfigRequest) (*ratelimit_pb.ClientConfigResponse, error) {
	_, rep, err := s.getClientConfig.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*ratelimit_pb.ClientConfigResponse), nil
}

func (s *grpcServer) UpdateClientConfig(ctx context.Context, req *ratelimit_pb.UpdateClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	_, rep, err := s.updateClientConfig.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*ratelimit_pb.StatusResponse), nil
}

func (s *grpcServer) ResetClientConfig(ctx context.Context, req *ratelimit_pb.ResetClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	_, rep, err := s.resetClientConfig.ServeGRPC(ctx, req)
	if err != nil {
		return nil, err
	}
	return rep.(*ratelimit_pb.StatusResponse), nil
}

// decodeGRPCGetClientConfigRequest is a transport/grpc.DecodeRequestFunc that converts a
// gRPC GetClientConfig request to a user-domain GetClientConfig request. Primarily useful in a server.
func decodeGRPCGetClientConfigRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*ratelimit_pb.GetClientConfigRequest)
	return endpoint.GetClientConfigRequest{Request: req}, nil
}

// encodeGRPCGetClientConfigResponse is a transport/grpc.EncodeResponseFunc that converts a
// user-domain GetClientConfig response to a gRPC GetClientConfig reply. Primarily useful in a server.
func encodeGRPCGetClientConfigResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoint.GetClientConfigResponse)
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Response, nil
}

// decodeGRPCUpdateClientConfigRequest is a transport/grpc.DecodeRequestFunc that converts a
// gRPC UpdateClientConfig request to a user-domain UpdateClientConfig request. Primarily useful in a server.
func decodeGRPCUpdateClientConfigRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*ratelimit_pb.UpdateClientConfigRequest)
	return endpoint.UpdateClientConfigRequest{Request: req}, nil
}

// encodeGRPCUpdateClientConfigResponse is a transport/grpc.EncodeResponseFunc that converts a
// user-domain UpdateClientConfig response to a gRPC UpdateClientConfig reply. Primarily useful in a server.
func encodeGRPCUpdateClientConfigResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoint.UpdateClientConfigResponse)
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Response, nil
}

// decodeGRPCResetClientConfigRequest is a transport/grpc.DecodeRequestFunc that converts a
// gRPC ResetClientConfig request to a user-domain ResetClientConfig request. Primarily useful in a server.
func decodeGRPCResetClientConfigRequest(_ context.Context, grpcReq interface{}) (interface{}, error) {
	req := grpcReq.(*ratelimit_pb.ResetClientConfigRequest)
	return endpoint.ResetClientConfigRequest{Request: req}, nil
}

// encodeGRPCResetClientConfigResponse is a transport/grpc.EncodeResponseFunc that converts a
// user-domain ResetClientConfig response to a gRPC ResetClientConfig reply. Primarily useful in a server.
func encodeGRPCResetClientConfigResponse(_ context.Context, response interface{}) (interface{}, error) {
	resp := response.(endpoint.ResetClientConfigResponse)
	if resp.Err != nil {
		return nil, resp.Err
	}
	return resp.Response, nil
}
