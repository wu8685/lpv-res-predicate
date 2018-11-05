package options

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Options struct {
	KubeConfig string
	Master     string
	Port       int

	ReservedCpuAnnoKey   string
	ReservedMemAnnoKey   string

	ConsiderUnboundLocalPV bool
}

func NewOptions() *Options {
	return &Options{
		Port: 8089,

		ReservedCpuAnnoKey: "reserved-cpu",
		ReservedMemAnnoKey: "reserved-mem",

		ConsiderUnboundLocalPV: true,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.KubeConfig, "kubeconfig", o.KubeConfig, "Path to a kubeconfig file, specifying how to connect to the API server. Providing --kubeconfig enables API server mode, omitting --kubeconfig enables standalone mode.")
	fs.StringVar(&o.Master, "master", o.Master, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	fs.IntVar(&o.Port, "listen-port", o.Port, "the port to listen to")
	fs.StringVar(&o.ReservedCpuAnnoKey, "reserved-cpu-annotation-key", o.ReservedCpuAnnoKey, "key of the annotation for information of reserved cpu of persistent local volume")
	fs.StringVar(&o.ReservedMemAnnoKey, "reserved-mem-annotation-key", o.ReservedMemAnnoKey, "key of the annotation for information of reserved memory of persistent local volume")
	fs.BoolVar(&o.ConsiderUnboundLocalPV, "consider-unbound-local-pv", o.ConsiderUnboundLocalPV, "whether the unbound local persistent volume will be considered")
}

func (o *Options) Validate() []error {
	errs := []error{}

	if len(o.KubeConfig) == 0 && len(o.Master) == 0 {
		errs = append(errs, fmt.Errorf("neither --kubeconfig nor --master is provided"))
	}

	if o.Port <= 0 {
		errs = append(errs, fmt.Errorf("--port %d should be greater than 0", o.Port))
	}
	return errs
}
