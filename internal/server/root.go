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
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"golang.nvidia.com/kaizen/nvkit/config"
)

var cfgFile string

func NewRootCommand() *cobra.Command {
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		log.Fatal("HOME environment variable is not set")
	}
	defaultConfigPath := fmt.Sprintf("%s/.nvkit/config.yaml", homeDir)

	rootCmd := &cobra.Command{
		Use:          "ratelimit-server",
		Short:        "Rate Limiting Server",
		Long:         `Rate Limiting Server with gRPC and HTTP endpoints.`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.InitConfig(cmd, cfgFile, defaultConfigPath, "ratelimit")
			return err
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultConfigPath, "config file (default is $HOME/.nvkit/config.yaml)")
	//grpc server
	rootCmd.AddCommand(makeServerCmd())

	return rootCmd
}

func Execute() {
	rootCmd := NewRootCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'\n", err)
		os.Exit(1)
	}
}
