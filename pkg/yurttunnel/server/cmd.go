/*
Copyright 2020 The OpenYurt Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"github.com/alibaba/openyurt/pkg/yurttunnel/constants"
	"github.com/alibaba/openyurt/pkg/yurttunnel/iptables"
	kubeutil "github.com/alibaba/openyurt/pkg/yurttunnel/kubernetes"
	"github.com/alibaba/openyurt/pkg/yurttunnel/pki"
	"github.com/alibaba/openyurt/pkg/yurttunnel/pki/certmanager"
	"github.com/alibaba/openyurt/pkg/yurttunnel/projectinfo"
)

// NewYurttunnelServerCommand creates a new yurttunnel-server command
func NewYurttunnelServerCommand(stopCh <-chan struct{}) *cobra.Command {
	o := NewYurttunnelServerOptions()

	cmd := &cobra.Command{
		Use:   "Launch " + projectinfo.GetServerName(),
		Short: projectinfo.GetServerName() + " sends requests to yurttunnel-agents",
		RunE: func(c *cobra.Command, args []string) error {
			if o.version {
				fmt.Println(projectinfo.ShortServerVersion())
				return nil
			}
			if err := o.validate(); err != nil {
				return err
			}
			if err := o.complete(); err != nil {
				return err
			}
			if err := o.run(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	flags.BoolVar(&o.version, "version", o.version,
		fmt.Sprintf("print the version information of the %s.",
			projectinfo.GetServerName()))
	flags.StringVar(&o.kubeConfig, "kube-config", o.kubeConfig,
		"path to the kubeconfig file.")
	flags.StringVar(&o.bindAddr, "bind-address", o.bindAddr,
		fmt.Sprintf("the ip address on which the %s will listen.",
			projectinfo.GetServerName()))
	flags.BoolVar(&o.enableIptables, "enable-iptables", o.enableIptables,
		"if allow iptable manager to set the dnat rule.")
	flags.IntVar(&o.iptablesSyncPeriod, "iptables-sync-period",
		o.iptablesSyncPeriod,
		"the synchronization period of the iptable manager.")
	return cmd
}

// YurttunnelServerOptions has the information that required by the
// yurttunel-server
type YurttunnelServerOptions struct {
	kubeConfig               string
	bindAddr                 string
	version                  bool
	enableIptables           bool
	iptablesSyncPeriod       int
	serverAgentPort          int
	serverMasterPort         int
	interceptorServerUDSFile string
	serverAgentAddr          string
	serverMasterAddr         string
	clientset                kubernetes.Interface
}

// NewYurttunnelServerOptions creates a new YurtNewYurttunnelServerOptions
func NewYurttunnelServerOptions() *YurttunnelServerOptions {
	o := &YurttunnelServerOptions{
		bindAddr:                 "0.0.0.0",
		enableIptables:           true,
		iptablesSyncPeriod:       60,
		serverAgentPort:          constants.YurttunnelServerAgentPort,
		serverMasterPort:         constants.YurttunnelServerMasterPort,
		interceptorServerUDSFile: "/tmp/interceptor-proxier.sock",
	}
	return o
}

// validate validates the YurttunnelServerOptions
func (o *YurttunnelServerOptions) validate() error {
	if len(o.bindAddr) == 0 {
		return fmt.Errorf("%s's bind address can't be empty",
			projectinfo.GetServerName())
	}
	return nil
}

// complete completes all the required options
func (o *YurttunnelServerOptions) complete() error {
	o.serverAgentAddr = fmt.Sprintf("%s:%d", o.bindAddr, o.serverAgentPort)
	o.serverMasterAddr = fmt.Sprintf("%s:%d", o.bindAddr, o.serverMasterPort)
	klog.Infof("server will accept %s requests at: %s, "+
		"server will accept master requests at: %s",
		projectinfo.GetAgentName(), o.serverAgentAddr, o.serverMasterAddr)
	var err error
	// As yurttunnel-server will run on the cloud, we use the function
	// 'kubeutil.CreateClientSet' to create the clientset, which will
	// try to create the clientset based on the in-cluster config if
	// the kubeconfig is empty.
	o.clientset, err = kubeutil.CreateClientSet(o.kubeConfig)
	return err
}

// run starts the yurttunel-server
func (o *YurttunnelServerOptions) run(stopCh <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 1. start the IP table manager
	if o.enableIptables {
		iptablesMgr := iptables.NewIptablesManager(o.clientset,
			o.bindAddr,
			o.iptablesSyncPeriod,
			stopCh)
		if iptablesMgr == nil {
			return fmt.Errorf("fail to create a new IptableManager")
		}
		iptablesMgr.Run()
	}

	// 2. create a certificate manager for the tunnel server and run the
	// csr approver for both yurttunnel-server and yurttunnel-agent
	serverCertMgr, err := certmanager.NewCertManager(
		o.clientset, constants.YurttunneServerCSROrg,
		constants.YurttunneServerCSRCN)
	if err != nil {
		return err
	}
	serverCertMgr.Start()
	go certmanager.ApproveYurttunnelCSR(o.clientset)

	// 3. get the latest certificate
	_ = wait.PollUntil(5*time.Second, func() (bool, error) {
		if serverCertMgr.Current() != nil {
			return true, nil
		}
		klog.Infof("waiting for the master to sign the %s certificate",
			projectinfo.GetServerName())
		return false, nil
	}, stopCh)

	// 4. generate the TLS configuration based on the latest certificate
	rootCertPool, err := pki.GenRootCertPool(o.kubeConfig,
		constants.YurttunnelCAFile)
	if err != nil {
		return fmt.Errorf("fail to generate the rootCertPool: %s", err)
	}
	tlsCfg, err :=
		pki.GenTLSConfigUseCertMgrAndCertPool(serverCertMgr, rootCertPool)
	if err != nil {
		return err
	}

	// 5. start the server
	if err := RunServer(ctx, o.interceptorServerUDSFile, o.serverMasterAddr,
		o.serverAgentAddr, tlsCfg); err != nil {
		return err
	}

	<-stopCh
	return nil
}
