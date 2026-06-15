/*
Copyright 2015 The Kubernetes Authors.

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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/krmcbride/k8s-leader-elector/internal/election"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	version  = "dev"
	revision = "unknown"

	name       = flag.String("election", "", "The name of the election")
	id         = flag.String("id", hostname(), "The id of this participant")
	namespace  = flag.String("election-namespace", corev1.NamespaceDefault, "The Kubernetes namespace for this election")
	ttl        = flag.Duration("ttl", 10*time.Second, "The TTL for this election")
	inCluster  = flag.Bool("use-cluster-credentials", false, "Use in-cluster Kubernetes credentials")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig file")
	addr       = flag.String("http", "", "If non-empty, stand up a simple webserver that reports the leader state")
	showVer    = flag.Bool("version", false, "Display version and exit")

	leaders = &leaderState{}
)

type leaderData struct {
	Name string `json:"name"`
}

type leaderState struct {
	mu   sync.RWMutex
	data leaderData
}

func (s *leaderState) Set(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Name = name
}

func (s *leaderState) Snapshot() leaderData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

func makeClient() (*kubernetes.Clientset, error) {
	cfg, err := restConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(rest.AddUserAgent(cfg, "leader-elector"))
}

func restConfig() (*rest.Config, error) {
	if *kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}

	if *inCluster {
		return rest.InClusterConfig()
	}

	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}

func webHandler(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(leaders.Snapshot())
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		_, _ = res.Write([]byte(err.Error()))
		return
	}

	res.WriteHeader(http.StatusOK)
	_, _ = res.Write(data)
}

func validateFlags() {
	if *id == "" {
		klog.Fatal("--id cannot be empty")
	}

	if *name == "" {
		klog.Fatal("--election cannot be empty")
	}

	if *ttl < time.Second {
		klog.Fatal("--ttl must be at least 1s")
	}
}

func main() {
	klog.InitFlags(flag.CommandLine)
	flag.Parse()

	if *showVer {
		fmt.Printf("leader-elector version=%s revision=%s\n", version, revision)
		return
	}

	validateFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	kubeClient, err := makeClient()
	if err != nil {
		klog.Fatalf("error connecting to the client: %v", err)
	}

	callback := func(leader string) {
		leaders.Set(leader)
		fmt.Printf("%s is the leader\n", leader)
	}

	elector, err := election.NewElection(*name, *id, *namespace, *ttl, callback, kubeClient)
	if err != nil {
		klog.Fatalf("failed to create election: %v", err)
	}

	go election.RunElection(ctx, elector)

	if *addr == "" {
		<-ctx.Done()
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", webHandler)
	server := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			klog.Errorf("error shutting down HTTP server: %v", err)
		}
	}()

	klog.Infof("http server starting at %s", *addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		klog.Fatalf("http server failed: %v", err)
	}
}

func hostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}
