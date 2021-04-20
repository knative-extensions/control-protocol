package scheduler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Scheduler is an interface that is able to allocate resources inside podsUid, following a strategy.
// The scheduler keeps track of where resources are allocated, but it doesn't manages the connections, nor send the actual messages,
// which the user still needs to do through the control.Service interface and the reconciler.ControlPlaneConnectionPool.
type Scheduler interface {
	// SyncPods make sure that the podsUid in the internal scheduler cache match the
	// actual running podsUid
	// TODO should this one be merged with Schedule?
	SyncPods(pods []string)

	// Schedule the object, returning the associated pod
	Schedule(object metav1.Object) string

	// DeallocateObject deallocates the object, that doesn't exist anymore
	DeallocateObject(uid types.UID)
}
