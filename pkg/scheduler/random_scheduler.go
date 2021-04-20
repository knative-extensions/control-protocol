package scheduler

import (
	"math/rand"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type randomScheduler struct {
	mutex sync.Mutex

	objects map[types.UID]metav1.Object

	// Map objects -> podsUid
	uidPods map[types.UID]string

	// Map of podsUid -> map of objects
	podsUid map[string]map[types.UID]interface{}

	// Enqueue
	enqueueKey func(name types.NamespacedName)
}

func NewRandomScheduler(enqueueKey func(name types.NamespacedName)) Scheduler {
	return &randomScheduler{
		objects:    make(map[types.UID]metav1.Object),
		uidPods:    make(map[types.UID]string),
		podsUid:    make(map[string]map[types.UID]interface{}),
		enqueueKey: enqueueKey,
	}
}

func (r *randomScheduler) SyncPods(pods []string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var toEnqueue []types.UID

	newPods := sets.NewString(pods...)
	oldPods := sets.StringKeySet(r.podsUid)

	deleted := oldPods.Difference(newPods)
	added := newPods.Difference(oldPods)

	for _, pod := range deleted.UnsortedList() {
		for uid := range r.podsUid[pod] {
			toEnqueue = append(toEnqueue, uid)
		}

		delete(r.podsUid, pod)
	}

	for _, pod := range added.UnsortedList() {
		r.podsUid[pod] = make(map[types.UID]interface{})
	}

	for _, uid := range toEnqueue {
		// We need to reschedule the ones
		// where the pod is not available anymore
		delete(r.uidPods, uid)

		obj := r.objects[uid]

		r.enqueueKey(types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		})
	}
}

func (r *randomScheduler) Schedule(object metav1.Object) string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	objectUid := object.GetUID()

	if p, ok := r.uidPods[objectUid]; ok {
		return p
	}

	// Random strategy: pick any pod!
	pods := sets.StringKeySet(r.podsUid).List()
	allocatedPod := pods[rand.Intn(len(pods))]

	r.objects[object.GetUID()] = object
	r.podsUid[allocatedPod][object.GetUID()] = struct{}{}
	r.uidPods[object.GetUID()] = allocatedPod

	return allocatedPod
}

func (r *randomScheduler) DeallocateObject(uid types.UID) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	allocatedPod, ok := r.uidPods[uid]
	if ok {
		delete(r.uidPods, uid)
		delete(r.podsUid[allocatedPod], uid)
		delete(r.objects, uid)
	}
}
