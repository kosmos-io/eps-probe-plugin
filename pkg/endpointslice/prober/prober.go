package prober

import (
	"time"

	"github.com/go-ping/ping"
	"k8s.io/klog/v2"

	"github.com/kosmos.io/eps-probe-plugin/pkg/endpointslice/prober/results"
)

func runProber(addresses []string) (map[string]results.Result, error) {
	result := map[string]results.Result{}
	for _, address := range addresses {
		pinger, err := ping.NewPinger(address)
		if err != nil {
			return nil, err
		}

		pinger.Count = 1
		pinger.Timeout = time.Second
		pinger.SetPrivileged(true)

		if err := pinger.Run(); err != nil {
			klog.ErrorS(err, "Run pinger failed.", "address", address)
			return nil, err
		}

		stats := pinger.Statistics()
		if stats.PacketsRecv >= 1 {
			result[address] = results.Success
			klog.V(5).InfoS("Ping success", "address", address)
		} else {
			result[address] = results.Failure
			klog.V(3).InfoS("Ping failed", "address", address)
		}
	}
	return result, nil
}
