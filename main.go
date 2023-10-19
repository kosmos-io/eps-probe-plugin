package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"

	"github.com/kosmos.io/eps-probe-plugin/pkg/serviceimport"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	clientgoscheme.AddToScheme(scheme) //nolint: errcheck

	v1alpha1.AddToScheme(scheme) //nolint: errcheck
}

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()

	var metricsAddr string
	var enableLeaderElection bool
	var probeFailureThreshold int
	var probePeriodSeconds int

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, ""+
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&probeFailureThreshold, "probe-failure-threshold", 3, "Minimum consecutive failure for the probe to be considered failed.")
	flag.IntVar(&probePeriodSeconds, "probe-period-seconds", 5, "How often (in seconds) to perform the probe.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:           scheme,
		Logger:           setupLog,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "eps-probe-plugin",
	})
	if err != nil {
		klog.ErrorS(err, "Could not new a controller manager")
		os.Exit(-1)
	}

	c := serviceimport.NewController(mgr.GetClient(), probePeriodSeconds, probeFailureThreshold)

	if err := (&serviceimport.Reconciler{Controller: c}).SetupWithManager(mgr); err != nil {
		klog.ErrorS(err, "Could not setup with manager")
		os.Exit(-1)
	}

	go c.Run(wait.NeverStop)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.ErrorS(err, "Manager exited non-zero")
		os.Exit(-1)
	}
}
