package election

import (
	"context"
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewElectionDoesNotContactAPIOnStartup(t *testing.T) {
	holder := "pod-a"
	client := fake.NewSimpleClientset(&coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{Name: "example", Namespace: metav1.NamespaceDefault},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity: &holder,
		},
	})
	var reported []string

	_, err := NewElection("example", "pod-b", metav1.NamespaceDefault, 10*time.Second, func(leader string) {
		reported = append(reported, leader)
	}, client)
	if err != nil {
		t.Fatalf("NewElection returned error: %v", err)
	}

	if len(reported) != 0 {
		t.Fatalf("reported leaders = %#v, want no startup callback", reported)
	}
}

func TestGetCurrentLeaderMissingLock(t *testing.T) {
	client := fake.NewSimpleClientset()

	leader, err := getCurrentLeader(context.Background(), "missing", metav1.NamespaceDefault, client)
	if err != nil {
		t.Fatalf("getCurrentLeader returned error: %v", err)
	}
	if leader != "" {
		t.Fatalf("leader = %q, want empty", leader)
	}
}
