package results

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

type Manager interface {
	// Get returns the cached result for the endpoint with the given serviceImport UID and endpoint address.
	Get(types.UID) (Result, bool)

	// Set sets the cached result for the endpoint with the given serviceImport UID and endpoint address.
	Set(*v1alpha1.ServiceImport, []string, Result)

	// Remove clears the cached result for the endpoint with the given serviceImport UID and endpoint address.
	Remove(types.UID)

	// Updates creates a channel that receives an Update whenever its result changes (but not removed).
	Updates() <-chan Update
}

type Result int

const (
	// Unknown is encoded as -1 (type Result)
	Unknown Result = iota - 1

	// Success is encoded as 0 (type Result)
	Success

	// Failure is encoded as 1 (type Result)
	Failure
)

type Update struct {
	Addresses     []string
	Result        Result
	SvcImportName string
	Namespace     string
}

// Manager implementation.
type manager struct {
	// guards the cache
	sync.RWMutex
	// map of Endpoint address -> probe Result
	cache map[types.UID]Result
	// channel of updates
	updates chan Update
}

var _ Manager = &manager{}

// NewManager create and returns an empty results manager.
func NewManager() Manager {
	return &manager{
		cache:   make(map[types.UID]Result),
		updates: make(chan Update, 20),
	}
}

func (m *manager) Get(id types.UID) (Result, bool) {
	m.RLock()
	defer m.RUnlock()
	result, found := m.cache[id]
	return result, found
}

func (m *manager) Set(svcImport *v1alpha1.ServiceImport, address []string, result Result) {
	if m.setInternal(svcImport.UID, result) {
		m.updates <- Update{address, result, svcImport.Name, svcImport.Namespace}
	}
}

func (m *manager) setInternal(id types.UID, result Result) bool {
	m.Lock()
	defer m.Unlock()
	prev, exists := m.cache[id]
	if !exists || prev != result {
		m.cache[id] = result
		return true
	}
	return false
}

func (m *manager) Remove(id types.UID) {
	m.Lock()
	defer m.Unlock()
	delete(m.cache, id)
}

func (m *manager) Updates() <-chan Update {
	return m.updates
}
