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

// Package election wraps Kubernetes client-go leader election for the sidecar binary.
package election

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

// NewSimpleElection creates an election with namespace defaulting to "default" and ttl to 10s.
func NewSimpleElection(electionID, id string, callback func(leader string), client kubernetes.Interface) (*leaderelection.LeaderElector, error) {
	return NewElection(electionID, id, metav1.NamespaceDefault, 10*time.Second, callback, client)
}

// NewElection creates a Lease-backed election. electionID is the Lease name and id should be unique per participant.
func NewElection(electionID, id, namespace string, ttl time.Duration, callback func(leader string), client kubernetes.Interface) (*leaderelection.LeaderElector, error) {
	leader, err := getCurrentLeader(context.Background(), electionID, namespace, client)
	if err != nil {
		return nil, err
	}
	callback(leader)

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      electionID,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	callbacks := leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			callback(id)
		},
		OnStoppedLeading: func() {
			leader, err := getCurrentLeader(context.Background(), electionID, namespace, client)
			if err != nil {
				klog.Errorf("failed to get leader: %v", err)
				callback("")
				return
			}
			callback(leader)
		},
		OnNewLeader: func(identity string) {
			callback(identity)
		},
	}

	config := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: ttl,
		RenewDeadline: ttl / 2,
		RetryPeriod:   ttl / 4,
		Callbacks:     callbacks,
	}

	return leaderelection.NewLeaderElector(config)
}

// RunElection runs an election until ctx is canceled, restarting the elector if leadership is lost.
func RunElection(ctx context.Context, elector *leaderelection.LeaderElector) {
	wait.UntilWithContext(ctx, elector.Run, 0)
}

func getCurrentLeader(ctx context.Context, electionID, namespace string, client kubernetes.Interface) (string, error) {
	lease, err := client.CoordinationV1().Leases(namespace).Get(ctx, electionID, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if lease.Spec.HolderIdentity == nil {
		return "", nil
	}
	return *lease.Spec.HolderIdentity, nil
}
