package serviceimport

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/eps-probe-plugin/pkg/endpointslice/prober"
	"github.com/kosmos.io/eps-probe-plugin/pkg/endpointslice/prober/results"
	"github.com/kosmos.io/eps-probe-plugin/pkg/serviceimport/annotation"
)

type Reconciler struct {
	Controller *Controller
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(3).InfoS("Reconcile", "serviceImport", req.NamespacedName)

	cleanup := false
	svcImport := &v1alpha1.ServiceImport{}
	if err := r.Controller.client.Get(ctx, req.NamespacedName, svcImport); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, err
		}
		cleanup = true
	}

	if cleanup || svcImport.DeletionTimestamp != nil {
		r.Controller.proberManager.RemoveServiceImport(req.NamespacedName.String())
		return ctrl.Result{}, nil
	}

	// Add the prober for the new serviceImport.
	if !r.Controller.proberManager.GetServiceImport(req.NamespacedName.String()) {
		r.Controller.proberManager.AddServiceImport(svcImport)
		return ctrl.Result{}, nil
	}

	// Update the prober for the serviceImport.
	if err := r.Controller.proberManager.UpdateServiceImport(svcImport); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	endpointSlicePredicate := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return true
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return true
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	},
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named("eps-probe").
		For(&v1alpha1.ServiceImport{}, endpointSlicePredicate).
		Complete(r)
}

type Controller struct {
	client            client.Client
	proberManager     prober.Manager
	resultsManager    results.Manager
	annotationManager annotation.Manager
}

func NewController(cli client.Client, periodSeconds, failureThreshold int) *Controller {
	resultsManager := results.NewManager()
	return &Controller{
		client:            cli,
		resultsManager:    resultsManager,
		proberManager:     prober.NewManager(resultsManager, periodSeconds, failureThreshold),
		annotationManager: annotation.NewManager(cli),
	}
}

const syncPeriod = 5 * time.Second

func (c *Controller) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()

	klog.InfoS("Staring eps-probe controller")
	defer klog.InfoS("Shutting down eps-probe controller")

	go c.annotationManager.Start()

	go wait.Until(c.syncLoop, syncPeriod, stopCh)

	<-stopCh
}

func (c *Controller) syncLoop() {
	select {
	case update := <-c.resultsManager.Updates():
		klog.V(3).InfoS("Received results", "results", update)
		c.annotationManager.Set("", update.Addresses, update.SvcImportName, update.Namespace)
	default:
	}
}
