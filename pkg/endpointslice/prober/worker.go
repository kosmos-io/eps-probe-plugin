package prober

import (
	"math/rand"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/eps-probe-plugin/pkg/endpointslice/prober/results"
)

type worker struct {
	// Channel for stopping the probe.
	stopCh chan struct{}

	// Channel for triggering the probe manually.
	manualTriggerCh chan struct{}

	// Channel for updating the endpointslice addresses.
	UpdateCh chan []string

	// Addresses to check the connectivity.
	addresses []string

	// The ServiceImport containing this probe.
	serviceImport *v1alpha1.ServiceImport

	// Where to store this workers results.
	resultsManager results.Manager
	probeManager   *manager

	// Describe the probe configuration.
	spec *probe

	// Records the addresses probe results.
	records      map[string]record
	latestResult results.Result
}

type probe struct {
	PeriodSeconds    int
	FailureThreshold int
}

type record struct {
	lastResult results.Result
	resultRun  int
}

func newWorker(m *manager, addrs []string, svcImport *v1alpha1.ServiceImport) *worker {
	w := &worker{
		stopCh:          make(chan struct{}, 1),
		manualTriggerCh: make(chan struct{}, 1),
		UpdateCh:        make(chan []string, 1),
		serviceImport:   svcImport,
		addresses:       addrs,
		probeManager:    m,
		resultsManager:  m.resultsManager,
		spec: &probe{
			PeriodSeconds:    m.spec.PeriodSeconds,
			FailureThreshold: m.spec.FailureThreshold,
		},
		records:      map[string]record{},
		latestResult: results.Success,
	}

	return w
}

func (w *worker) run() {
	probeTickerPeriod := time.Duration(w.spec.PeriodSeconds) * time.Second

	if probeTickerPeriod > time.Since(w.probeManager.start) {
		time.Sleep(time.Duration(rand.Float64() * float64(probeTickerPeriod))) //nolint: gosec
	}

	probeTicker := time.NewTicker(probeTickerPeriod)
	defer func() {
		probeTicker.Stop()
		namespaceName := w.serviceImport.Namespace + string(types.Separator) + w.serviceImport.Name
		w.probeManager.removeWorker(namespaceName)
	}()

probeLoop:
	for {
		select {
		case <-w.stopCh:
			klog.V(3).InfoS("Stopping prober worker", "serviceImport", klog.KObj(w.serviceImport))
			break probeLoop
		case <-probeTicker.C:
			w.doProbe()
		case <-w.manualTriggerCh:
		case updates := <-w.UpdateCh:
			w.addresses = updates
		}
	}
}

func (w *worker) stop() {
	select {
	case w.stopCh <- struct{}{}:
	default: // Non-blocking.
	}
}

// doProbe probes the endpointSlice once and records the result.
// Returns whether the worker should continue.
func (w *worker) doProbe() (keepGoing bool) {
	defer func() { recover() }() //nolint: errcheck // Actually eat panics (HandleCrash takes care of logging)
	defer runtime.HandleCrash(func(_ interface{}) { keepGoing = true })

	if w.serviceImport.DeletionTimestamp != nil {
		klog.V(3).InfoS("ServiceImport deletion requested, setting probe result to success",
			"serviceImport", klog.KObj(w.serviceImport))

		w.resultsManager.Set(w.serviceImport, w.addresses, results.Success)

		return false
	}

	result, err := runProber(w.addresses)
	if err != nil {
		return true
	}

	// Store the probe results into w.records
	for address, r := range result {
		if w.records[address].lastResult == r {
			w.records[address] = record{
				resultRun:  w.records[address].resultRun + 1,
				lastResult: r,
			}
		} else {
			w.records[address] = record{
				resultRun:  1,
				lastResult: r,
			}
		}
	}

	// Check if the number of failures has been reached.
	var addrs []string
	for addr, r := range w.records {
		if r.lastResult == results.Failure && r.resultRun >= w.spec.FailureThreshold {
			addrs = append(addrs, addr)
		}
	}

	if len(addrs) == 0 && w.latestResult == results.Failure {
		w.resultsManager.Set(w.serviceImport, w.addresses, results.Success)
		w.latestResult = results.Success
		klog.V(3).InfoS("Set probe results to success", "serviceImport", klog.KObj(w.serviceImport))
	}
	if len(addrs) != 0 && w.latestResult == results.Success {
		w.resultsManager.Set(w.serviceImport, addrs, results.Failure)
		w.latestResult = results.Failure
		klog.V(3).InfoS("Set probe results to failure", "serviceImport", klog.KObj(w.serviceImport), "not reachable addresses", addrs)
	}

	return true
}
