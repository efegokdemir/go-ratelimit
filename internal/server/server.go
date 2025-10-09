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
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/endpoint"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/service"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/internal/ratelimit/transport"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/config"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/limiter"
	"gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/pkg/logger"

	_ "google.golang.org/grpc/health"

	_ "golang.nvidia.com/kaizen/nvkit/config"

	"github.com/go-kit/kit/metrics"
	"github.com/go-kit/kit/metrics/prometheus"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"golang.nvidia.com/kaizen/nvkit/auth"
	"golang.nvidia.com/kaizen/nvkit/clients"
	"golang.nvidia.com/kaizen/nvkit/logs"
	nvkitmetrics "golang.nvidia.com/kaizen/nvkit/metrics"
	"golang.nvidia.com/kaizen/nvkit/servers"

	ratelimit_pb "gitlab-master.nvidia.com/kratos-dataverse-observability/go-ratelimit/api/nvidia/v1"
)

const namespace = "kratos"
const serviceName = "kratos-dataverse-observability"

type UAMAuthzConfig struct {
	Enabled          bool
	Namespace        string
	PolicyFQDN       string
	UAMEvaluatorAddr string
	SSAServiceAddr   string
	SSAClientID      string
	SSAClientSecret  string
}

type grpcCfg struct {
	*servers.GRPCConfig
	AuthzCfg        UAMAuthzConfig
	EnableJWTParser bool
	JWKSetURL       string
}

func makeServerCmd() *cobra.Command {

	serverConfig := &grpcCfg{
		GRPCConfig:      &servers.GRPCConfig{BaseServerConfig: &servers.BaseServerConfig{ServiceName: serviceName}},
		AuthzCfg:        UAMAuthzConfig{},
		EnableJWTParser: false,
	}
	cmd := &cobra.Command{
		Use:        "server ",
		Aliases:    nil,
		SuggestFor: nil,
		Short:      "grpc api server command",
		Long: `grpc api server command.
Usage:
  ratelimit-server server --help // for usage
  ratelimit-server server --config <config-file>
`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverCfg := serverConfig.GRPCConfig

			//cfg, _ := config.LoadConfig(cfgFile)

			// Create a single logger, which we'll use and give to other components.
			atom := zap.NewAtomicLevel()
			zaplogger := logs.NewZapLogger(atom)
			fmt.Println(cfgFile)
			cfg, _ := config.LoadConfig(cfgFile)
			logger.InitLogger(cfg)

			// Setup standard request duration histogram
			duration := nvkitmetrics.RequestDurationHistogram(namespace, serviceName)

			// Create the (sparse) metrics we'll use in the service. They, too, are
			// dependencies that we pass to components that use them.
			var kdoCtr metrics.Counter
			{
				// Business-level metrics.
				kdoCtr = prometheus.NewCounterFrom(
					stdprometheus.CounterOpts{
						Namespace: "kratos",
						Subsystem: "dataverse_obs",
						Name:      "num_called",
						Help:      "Total number of times the service was called.",
					}, []string{},
				)
			}
			var authzClient clients.UAMAuthZClientInterface
			var err error

			if serverConfig.AuthzCfg.Enabled {
				if len(serverConfig.AuthzCfg.SSAClientID) == 0 {
					return fmt.Errorf("undefined --authz.uam.ssa-client-id or env NVKIT_RATelimit_AUTHZ_UAM_SSA_CLIENT_ID")
				}
				if len(serverConfig.AuthzCfg.SSAClientSecret) == 0 {
					return fmt.Errorf("undefined --authz.uam.ssa-client-secret or env NVKIT_RATelimit_AUTHZ_UAM_SSA_CLIENT_SECRET")
				}

				authzClient, err = clients.NewUAMAuthzClient(
					&clients.BaseClientConfig{
						Type: "http",
						Addr: serverConfig.AuthzCfg.UAMEvaluatorAddr,
						TLS:  auth.TLSConfigOptions{},
						// Tracer: tracer,
						AuthnCfg: &auth.AuthnConfig{
							OIDCConfig: &auth.ProviderConfig{
								Host:         serverConfig.AuthzCfg.SSAServiceAddr,
								ClientID:     serverConfig.AuthzCfg.SSAClientID,
								ClientSecret: serverConfig.AuthzCfg.SSAClientSecret,
								Scopes:       []string{"pdp-evaluate"},
							},
						},
					}, &auth.UAMPolicyConfig{
						Namespace:  serverConfig.AuthzCfg.Namespace,
						PolicyFQDN: serverConfig.AuthzCfg.PolicyFQDN,
					},
				)
				if err != nil {
					zaplogger.L().Error("cannot initialize authz client", zap.Error(err))
					return err
				}
			}

			// Create a dynamic limiter for the service
			dynamicLimiter := limiter.NewDynamicLimiter(
				nil, // No Redis for now, can be added later
				"ratelimit",
				100,     // Default request limit
				1048576, // Default byte limit (1MB)
				time.Minute,
				func(r *http.Request) string { return "default-client" },
			)

			var (
				service    = service.New(zaplogger, kdoCtr, &cfg, dynamicLimiter)
				endpoints  = endpoint.New(service, authzClient, serverConfig.JWKSetURL, duration, serverConfig.EnableJWTParser)
				grpcServer = transport.NewGRPCServer(endpoints, zaplogger)
			)

			registerServerFunc := func(s *grpc.Server) {
				s.RegisterService(&ratelimit_pb.AdminService_ServiceDesc, grpcServer)
			}

			configDumpFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				configData, err := json.MarshalIndent(serverConfig, "", "  ")
				if err != nil {
					zaplogger.L().Error("config_dump marshal error", zap.Error(err))
					w.Write([]byte("Failed to marshal config"))
				}
				w.Write(configData)
			})

			// Define handlers for the admin port (8081)
			adminHandlers := map[string]http.Handler{
				"/log_level":   atom,
				"/config_dump": configDumpFunc,
			}

			svrOpts := []servers.Option{
				servers.WithLogger(zaplogger),
				servers.WithRegisterServer(registerServerFunc),
				servers.WithRegisterHandler(ratelimit_pb.RegisterAdminServiceHandler),
				servers.WithAdditionalHTTPHandlers(adminHandlers),
			}

			fmt.Println(serverCfg.GRPCAddr)
			svr := servers.NewGRPCServer(serverCfg, svrOpts...)
			if err := svr.Setup(); err != nil {
				zaplogger.Log("err: ", err)
				return err
			}

			zaplogger.Log("exit", svr.Run())

			return nil
		},
	}
	serverConfig.AddCommandFlags(cmd)
	return cmd
}

func (c *grpcCfg) AddCommandFlags(cmd *cobra.Command) {
	c.GRPCConfig.AddServerFlags(cmd)
	cmd.Flags().BoolVarP(&c.EnableJWTParser, "authn.enabled", "", false, "Set to true to allow for jwt token validation")
	cmd.Flags().StringVarP(&c.JWKSetURL, "authn.jwk-set-url", "", "", "JWK set URL - https://[stg.]login.nvidia.com/.well-known/jwks.json | https://<SERVICE_ID>[.stg].ssa.nvidia.com/.well-known/jwks.json\n"+
		"If passing `idToken` from Starfleet, set jwk-set-url to https://[stg.]login.nvidia.com/.well-known/jwks.json\n"+
		"If passing `accessToken` from SSA, SERVICE_ID in jwt-set-url can be fetched from https://admin[.stg].login.nvidia.com/\n"+
		"	Lookup your service -> View service details (gear icon) -> Service ID")
	cmd.Flags().BoolVarP(&c.AuthzCfg.Enabled, "authz.uam.enabled", "", false, "Set to true to allow for uam authz evaluation")
	cmd.Flags().StringVarP(&c.AuthzCfg.Namespace, "authz.uam.namespace", "", "uamdemo", "Authz UAM namespace")
	cmd.Flags().StringVarP(&c.AuthzCfg.PolicyFQDN, "authz.uam.policy-fqdn", "", "ratelimit.allow", "Authz UAM Policy FQDN")
	cmd.Flags().StringVarP(&c.AuthzCfg.SSAServiceAddr, "authz.uam.ssa-addr", "", "https://0ksgdfuwnpgbtbhaozzo4l43onuuvo1ol-jbct30rhq.stg.ssa.nvidia.com", "UAM service host")
	cmd.Flags().StringVarP(&c.AuthzCfg.SSAClientID, "authz.uam.ssa-client-id", "", "", "UAM authorized SSA Client ID. (Env: UAM_SSA_CLIENT_ID)")
	cmd.Flags().StringVarP(&c.AuthzCfg.SSAClientSecret, "authz.uam.ssa-client-secret", "", "", "UAM authorized SSA Client Secret (Env: UAM_SSA_CLIENT_SECRET)")
	cmd.Flags().StringVarP(&c.AuthzCfg.UAMEvaluatorAddr, "authz.uam.evaluator-addr", "", "https://pdp.stg.uam.nvidia.com", "UAM evaluator address")
}
