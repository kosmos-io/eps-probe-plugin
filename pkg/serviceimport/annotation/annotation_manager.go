package annotation

import (
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

type Manager interface {
	Start()

	Set(uid types.UID, addrs []string, svcImportName, svcImportNamespace string)

	syncAnnotation(uid types.UID, status annotationStatus)
}

type annotationStatus struct {
	Addresses     []string
	SvcImportName string
	Namespace     string
}

type serviceImportAnnotationSyncRequest struct {
	serviceImportUID types.UID
	status           annotationStatus
}

type manager struct {
	client client.Client

	serviceImportAnnotationChannel chan serviceImportAnnotationSyncRequest
}

func NewManager(client client.Client) Manager {
	return &manager{
		client:                         client,
		serviceImportAnnotationChannel: make(chan serviceImportAnnotationSyncRequest, 1000),
	}
}

const syncPeriod = 10 * time.Second

func (m *manager) Start() {
	if m.client == nil {
		klog.InfoS("kubernetes client is nil, not starting status manager")
		return
	}

	klog.InfoS("Starting to sync serviceImport annotation with apiserver")

	syncTicker := time.NewTicker(syncPeriod)

	// syncServiceImport and syncBatch share the same go routine to avoid sync races.
	go wait.Forever(func() {
		for {
			select {
			case syncRequest := <-m.serviceImportAnnotationChannel:
				klog.V(3).InfoS("Annotation manager: syncing serviceImport with status from serviceImportAnnotationChannel",
					"serviceImportUID", syncRequest.serviceImportUID)
				m.syncAnnotation(syncRequest.serviceImportUID, syncRequest.status)
			case <-syncTicker.C:
			}
		}
	}, 0)
}

func (m *manager) Set(uid types.UID, addrs []string, svcImportName, svcImportNamespace string) {
	m.serviceImportAnnotationChannel <- serviceImportAnnotationSyncRequest{
		serviceImportUID: uid,
		status: annotationStatus{
			Addresses:     addrs,
			SvcImportName: svcImportName,
			Namespace:     svcImportNamespace,
		},
	}
}

const (
	ServiceImportNotReachableEPSAddr = "kosmos.io/disconnected-address"
)

func (m *manager) syncAnnotation(uid types.UID, status annotationStatus) {
	svcImport := &v1alpha1.ServiceImport{}
	if err := m.client.Get(context.TODO(), client.ObjectKey{
		Name:      status.SvcImportName,
		Namespace: status.Namespace,
	}, svcImport); err != nil {
		klog.V(3).ErrorS(err, "Could not get serviceImport annotation", "serviceImport", klog.KObj(svcImport))
		return
	}

	if svcImport.DeletionTimestamp != nil {
		return
	}

	svcImport.Annotations[ServiceImportNotReachableEPSAddr] = strings.Join(status.Addresses, ",")
	if err := m.client.Update(context.TODO(), svcImport); err != nil {
		klog.V(3).ErrorS(err, "Could not update serviceImport annotation", "serviceImport", klog.KObj(svcImport))
		return
	}
	klog.V(3).InfoS("Success to update serviceImport annotation", "serviceImport", klog.KObj(svcImport))
}
