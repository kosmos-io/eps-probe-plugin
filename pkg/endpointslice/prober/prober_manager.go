package prober

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/eps-probe-plugin/pkg/endpointslice/prober/results"
	"github.com/kosmos.io/eps-probe-plugin/pkg/util"
)

const (
	ServiceImportEPSAddr = "kosmos.io/address"
)

type Manager interface {
	// AddServiceImport creates new probe workers for every ServiceImport probe. This should be called for every
	// ServiceImport created.
	AddServiceImport(svcImport *v1alpha1.ServiceImport)

	// GetServiceImport checks if the probe workers has been created.
	GetServiceImport(svcImport *v1alpha1.ServiceImport) bool

	// UpdateService sends UpdateChan to worker.
	UpdateServiceImport(svcImport *v1alpha1.ServiceImport) error

	// RemoveServiceImport handles cleaning up the removed ServiceImport.
	RemoveServiceImport(svcImport *v1alpha1.ServiceImport)

	// CleanupServiceImport handles cleaning up ServiceImport which should no logger be existed.
	CleanupServiceImports(desiredSvcImports map[types.UID]sets.Empty)
}

// NewManager creates a Manager for serviceImport and endpointSlice probing.
func NewManager(resultsManager results.Manager, periodSeconds, failureThreshold int) Manager {
	return &manager{
		workers:        make(map[probeKey]*worker),
		start:          clock.RealClock{}.Now(),
		resultsManager: resultsManager,
		spec: probeSpec{
			PeriodSeconds:    periodSeconds,
			FailureThreshold: failureThreshold,
		},
	}
}

type manager struct {
	// Map of active workers for probes
	workers map[probeKey]*worker

	// Lock for accessing & mutating workers
	workerLock sync.RWMutex

	// resultsManager manages the results of probes
	resultsManager results.Manager

	spec probeSpec

	start time.Time
}

type probeSpec struct {
	PeriodSeconds    int
	FailureThreshold int
}

type probeKey struct {
	serviceImportUID types.UID
}

func (m *manager) AddServiceImport(svcImport *v1alpha1.ServiceImport) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()

	key := probeKey{serviceImportUID: svcImport.UID}
	if _, ok := m.workers[key]; ok {
		klog.ErrorS(nil, "Probe already exists for serviceImport", "serviceImport", klog.KObj(svcImport))
		return
	}

	addrs, err := util.ConvertStringToAddresses(svcImport.Annotations[ServiceImportEPSAddr])
	if err != nil {
		klog.ErrorS(err, "Can't parse ips from annotations", "serviceImport", klog.KObj(svcImport))
		return
	}

	w := newWorker(m, addrs, svcImport)
	m.workers[key] = w
	go w.run()
}

func (m *manager) GetServiceImport(svcImport *v1alpha1.ServiceImport) bool {
	_, ok := m.getWorker(svcImport.UID)
	return ok
}

func (m *manager) UpdateServiceImport(svcImport *v1alpha1.ServiceImport) error {
	desired, err := util.ConvertStringToAddresses(svcImport.Annotations[ServiceImportEPSAddr])
	if err != nil {
		klog.ErrorS(err, "Can't parse ips from annotations", "serviceImport", klog.KObj(svcImport))
		return err
	}

	worker, ok := m.getWorker(svcImport.UID)
	if !ok {
		klog.ErrorS(nil, "Probe does not exists for serviceImport", "serviceImport", klog.KObj(svcImport))
		return fmt.Errorf("ProbeNotFound")
	}
	current := worker.addresses

	sort.Strings(desired)
	sort.Strings(current)
	if !reflect.DeepEqual(current, desired) {
		m.workers[probeKey{serviceImportUID: svcImport.UID}].UpdateCh <- desired
	}
	return nil
}

func (m *manager) RemoveServiceImport(svcImport *v1alpha1.ServiceImport) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()

	klog.V(3).InfoS("Removing serviceImport from prober manager", "serviceImport", klog.KObj(svcImport))

	key := probeKey{serviceImportUID: svcImport.UID}
	m.workers[key].stop()
}

func (m *manager) CleanupServiceImports(desiredSvcImports map[types.UID]sets.Empty) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()

	for key, worker := range m.workers {
		if _, ok := desiredSvcImports[key.serviceImportUID]; !ok {
			worker.stop()
		}
	}
}

func (m *manager) getWorker(svcImportUID types.UID) (*worker, bool) {
	m.workerLock.RLock()
	defer m.workerLock.RUnlock()
	worker, ok := m.workers[probeKey{serviceImportUID: svcImportUID}]
	return worker, ok
}

// removeWorker called by the worker after exiting.
func (m *manager) removeWorker(svcImportUID types.UID) {
	m.workerLock.Lock()
	defer m.workerLock.Unlock()
	delete(m.workers, probeKey{serviceImportUID: svcImportUID})
}
