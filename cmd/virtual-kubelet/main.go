// Copyright © 2017 The virtual-kubelet authors
// Copyright © 2020 Elotl Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"strings"

	"github.com/elotl/cloud-instance-provider/pkg/glog"
	"github.com/elotl/cloud-instance-provider/pkg/server"
	cli "github.com/elotl/node-cli"
	opencensuscli "github.com/elotl/node-cli/opencensus"
	"github.com/elotl/node-cli/opts"
	"github.com/elotl/node-cli/provider"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
)

var (
	buildVersion = "N/A"
	buildTime    = "N/A"
	k8sVersion   = "v1.14.0" // This should follow the version of k8s.io/kubernetes we are importing
)

func main() {
	ctx := cli.ContextWithCancelOnSignal(context.Background())

	log.L = glog.NewGlogAdapter()

	trace.T = opencensus.Adapter{}
	traceConfig := opencensuscli.Config{
		AvailableExporters: map[string]opencensuscli.ExporterInitFunc{
			"ocagent": initOCAgent,
		},
	}

	serverConfig := &ServerConfig{}

	o, err := opts.FromEnv()
	if err != nil {
		log.G(ctx).Fatal(err)
	}
	o.Provider = "cloud-instance-provider"
	o.Version = strings.Join([]string{k8sVersion, "vk", buildVersion}, "-")
	o.PodSyncWorkers = 10

	node, err := cli.New(ctx,
		cli.WithBaseOpts(o),
		cli.WithCLIVersion(buildVersion, buildTime),
		cli.WithProvider("cloud-instance-provider",
			func(cfg provider.InitConfig) (provider.Provider, error) {
				return server.NewInstanceProvider(
					cfg.ConfigPath,
					cfg.NodeName,
					cfg.InternalIP,
					cfg.DaemonPort,
					serverConfig.DebugServer,
					cfg.ResourceManager,
					ctx.Done(),
				)
			}),
		cli.WithPersistentFlags(traceConfig.FlagSet()),
		cli.WithPersistentFlags(serverConfig.FlagSet()),
		cli.WithPersistentPreRunCallback(func() error {
			return opencensuscli.Configure(ctx, &traceConfig, o)
		}),
	)

	if err != nil {
		log.G(ctx).Fatal(err)
	}

	if err := node.Run(); err != nil {
		log.G(ctx).Fatal(err)
	}
}
