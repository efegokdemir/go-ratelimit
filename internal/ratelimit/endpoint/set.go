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
package endpoint

import (
	"context"
	"encoding/json"

	_ "google.golang.org/protobuf/types/known/emptypb"

	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/model"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/service"

	"github.com/go-kit/kit/endpoint"
	kitmetrics "github.com/go-kit/kit/metrics"
	otelkit "go.opentelemetry.io/contrib/instrumentation/github.com/go-kit/kit/otelkit"
	"go.uber.org/zap"
	"golang.nvidia.com/kaizen/nvkit/middleware/logs"
	"golang.nvidia.com/kaizen/nvkit/middleware/metrics"

	pdpv1 "golang.nvidia.com/kaizen/nvkit/api/pdp/v1"
	"golang.nvidia.com/kaizen/nvkit/clients"
	"golang.nvidia.com/kaizen/nvkit/middleware/authz"
)

// Set collects all of the endpoints that compose an hello service. It's meant to
// be used as a helper struct, to collect all of the endpoints into a single
// parameter.
type Set struct {
	// Admin API endpoints
	GetClientConfigEndpoint    endpoint.Endpoint
	UpdateClientConfigEndpoint endpoint.Endpoint
	ResetClientConfigEndpoint  endpoint.Endpoint
}

// New returns a Set that wraps the provided server, and wires in all of the
// expected endpoint middlewares via the various parameters.
func New(svc service.Service, authzClient clients.UAMAuthZClientInterface, authnJwtSetURL string, duration kitmetrics.Histogram, enableJWTParser bool) Set {
	addAuthzMiddlwareFn := func(ep endpoint.Endpoint, requestType string) endpoint.Endpoint {
		if authzClient != nil {
			marshaler := func(request interface{}) (map[string]interface{}, error) {
				switch requestType {
				case "GetClientConfig":
					req := request.(GetClientConfigRequest)
					return map[string]interface{}{
						"client_id": req.Request.ClientId,
						"action":    "read",
					}, nil
				case "UpdateClientConfig":
					req := request.(UpdateClientConfigRequest)
					return map[string]interface{}{
						"client_id": req.Request.ClientId,
						"action":    "update",
						"config": map[string]interface{}{
							"request_limit": req.Request.Config.RequestLimit,
							"byte_limit":    req.Request.Config.ByteLimit,
							"window_secs":   req.Request.Config.WindowSecs,
						},
					}, nil
				case "ResetClientConfig":
					req := request.(ResetClientConfigRequest)
					return map[string]interface{}{
						"client_id": req.Request.ClientId,
						"action":    "delete",
					}, nil
				default:
					return map[string]interface{}{
						"action": "unknown",
					}, nil
				}
			}

			// This custom response handler takes the evaluated pdp response and adds to context a greeting message
			authzResponseHandler := func(ctx context.Context, response *pdpv1.RuleResponse) (context.Context, bool) {
				respBytes, err := json.Marshal(response.Result)
				if err != nil {
					zap.L().Info("response marshal failed", zap.Error(err))
					return ctx, false
				}

				var authzResp model.AuthzResponse
				err = json.Unmarshal(respBytes, &authzResp)
				if err != nil {
					zap.L().Info("response unmarshal failed", zap.Error(err))
					return nil, false
				}

				newCtx := context.WithValue(ctx, "x-authz-greeting", authzResp.Meta.CustomGreet)
				return newCtx, authzResp.Allow
			}
			ep = authz.UAMPolicyAuthorizer(authzClient, marshaler, authzResponseHandler)(ep)
		}
		return ep
	}

	var GetClientConfigEndpoint endpoint.Endpoint
	{
		GetClientConfigEndpoint = MakeGetClientConfigEndpoint(svc)
		GetClientConfigEndpoint = addAuthzMiddlwareFn(GetClientConfigEndpoint, "GetClientConfig")
		GetClientConfigEndpoint = logs.ZapEndpointLogger((&logs.KV{}).With("method", "GetClientConfig"))(GetClientConfigEndpoint)
		GetClientConfigEndpoint = metrics.RequestDurationInstrumentor(duration.With("method", "GetClientConfig"))(GetClientConfigEndpoint)
		GetClientConfigEndpoint = otelkit.EndpointMiddleware(otelkit.WithOperation("GetClientConfig"))(GetClientConfigEndpoint)
	}

	var UpdateClientConfigEndpoint endpoint.Endpoint
	{
		UpdateClientConfigEndpoint = MakeUpdateClientConfigEndpoint(svc)
		UpdateClientConfigEndpoint = addAuthzMiddlwareFn(UpdateClientConfigEndpoint, "UpdateClientConfig")
		UpdateClientConfigEndpoint = logs.ZapEndpointLogger((&logs.KV{}).With("method", "UpdateClientConfig"))(UpdateClientConfigEndpoint)
		UpdateClientConfigEndpoint = metrics.RequestDurationInstrumentor(duration.With("method", "UpdateClientConfig"))(UpdateClientConfigEndpoint)
		UpdateClientConfigEndpoint = otelkit.EndpointMiddleware(otelkit.WithOperation("UpdateClientConfig"))(UpdateClientConfigEndpoint)
	}

	var ResetClientConfigEndpoint endpoint.Endpoint
	{
		ResetClientConfigEndpoint = MakeResetClientConfigEndpoint(svc)
		ResetClientConfigEndpoint = addAuthzMiddlwareFn(ResetClientConfigEndpoint, "ResetClientConfig")
		ResetClientConfigEndpoint = logs.ZapEndpointLogger((&logs.KV{}).With("method", "ResetClientConfig"))(ResetClientConfigEndpoint)
		ResetClientConfigEndpoint = metrics.RequestDurationInstrumentor(duration.With("method", "ResetClientConfig"))(ResetClientConfigEndpoint)
		ResetClientConfigEndpoint = otelkit.EndpointMiddleware(otelkit.WithOperation("ResetClientConfig"))(ResetClientConfigEndpoint)
	}

	return Set{

		GetClientConfigEndpoint:    GetClientConfigEndpoint,
		UpdateClientConfigEndpoint: UpdateClientConfigEndpoint,
		ResetClientConfigEndpoint:  ResetClientConfigEndpoint,
	}
}

// GetClientConfigRequest collects the request parameters for the GetClientConfig method.
type GetClientConfigRequest struct {
	Request *ratelimit_pb.GetClientConfigRequest
}

// GetClientConfigResponse collects the response values for the GetClientConfig method.
type GetClientConfigResponse struct {
	Response *ratelimit_pb.ClientConfigResponse
	Err      error
}

// GetClientConfig implements the service interface.
func (s Set) GetClientConfig(ctx context.Context, req *ratelimit_pb.GetClientConfigRequest) (*ratelimit_pb.ClientConfigResponse, error) {
	resp, err := s.GetClientConfigEndpoint(ctx, GetClientConfigRequest{Request: req})
	if err != nil {
		return nil, err
	}
	response := resp.(GetClientConfigResponse)
	return response.Response, response.Err
}

// MakeGetClientConfigEndpoint constructs a GetClientConfig endpoint wrapping the service.
func MakeGetClientConfigEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(GetClientConfigRequest)
		result, err := s.GetClientConfig(ctx, req.Request)
		return GetClientConfigResponse{Response: result, Err: err}, err
	}
}

// UpdateClientConfigRequest collects the request parameters for the UpdateClientConfig method.
type UpdateClientConfigRequest struct {
	Request *ratelimit_pb.UpdateClientConfigRequest
}

// UpdateClientConfigResponse collects the response values for the UpdateClientConfig method.
type UpdateClientConfigResponse struct {
	Response *ratelimit_pb.StatusResponse
	Err      error
}

// UpdateClientConfig implements the service interface.
func (s Set) UpdateClientConfig(ctx context.Context, req *ratelimit_pb.UpdateClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	resp, err := s.UpdateClientConfigEndpoint(ctx, UpdateClientConfigRequest{Request: req})
	if err != nil {
		return nil, err
	}
	response := resp.(UpdateClientConfigResponse)
	return response.Response, response.Err
}

// MakeUpdateClientConfigEndpoint constructs a UpdateClientConfig endpoint wrapping the service.
func MakeUpdateClientConfigEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(UpdateClientConfigRequest)
		result, err := s.UpdateClientConfig(ctx, req.Request)
		return UpdateClientConfigResponse{Response: result, Err: err}, err
	}
}

// ResetClientConfigRequest collects the request parameters for the ResetClientConfig method.
type ResetClientConfigRequest struct {
	Request *ratelimit_pb.ResetClientConfigRequest
}

// ResetClientConfigResponse collects the response values for the ResetClientConfig method.
type ResetClientConfigResponse struct {
	Response *ratelimit_pb.StatusResponse
	Err      error
}

// ResetClientConfig implements the service interface.
func (s Set) ResetClientConfig(ctx context.Context, req *ratelimit_pb.ResetClientConfigRequest) (*ratelimit_pb.StatusResponse, error) {
	resp, err := s.ResetClientConfigEndpoint(ctx, ResetClientConfigRequest{Request: req})
	if err != nil {
		return nil, err
	}
	response := resp.(ResetClientConfigResponse)
	return response.Response, response.Err
}

// MakeResetClientConfigEndpoint constructs a ResetClientConfig endpoint wrapping the service.
func MakeResetClientConfigEndpoint(s service.Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(ResetClientConfigRequest)
		result, err := s.ResetClientConfig(ctx, req.Request)
		return ResetClientConfigResponse{Response: result, Err: err}, err
	}
}
