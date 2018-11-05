package app

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/kubernetes/pkg/version/verflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
	"github.com/wu8685/lpv-res-predicate/cmd/predicate-server/app/options"
	"github.com/wu8685/lpv-res-predicate/pkg"
)

func NewPredicateServerCommand() *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use: "predicate-server",
		Long: `The pv resource predicate server is called by kube-scheduler HTTP extender
to predicate whether a group of nodes is fit for a pod. Because a node has to reserve resources (cpu and memory)
for pods using local persistent volume on this node, it predicates a node by considering
whether the node still has enough resources (cpu and memory) for sequential pods using local PV
after it provisions the current pod.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()

			if len(args) != 0 {
				fmt.Fprint(os.Stderr, "Arguments are not supported\n")
			}

			if errs := opts.Validate(); len(errs) > 0 {
				fmt.Fprintf(os.Stderr, "%v\n", utilerrors.NewAggregate(errs))
				os.Exit(1)
			}

			if err := Run(opts); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}

	opts.AddFlags(cmd.Flags())

	return cmd
}

func Run(opts *options.Options) error {
	cfg, err := newConfig(opts)
	if err != nil {
		return err
	}

	svr := pkg.NewPredicateServer(cfg)
	return svr.Run()
}

func newConfig(opts *options.Options) (*config.Config, error) {
	cfg, err := clientcmd.BuildConfigFromFlags(opts.Master, opts.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes clientset: %s", err.Error())
	}

	informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second*30)

	options := &config.OptionConfig{
		ListenPort:             opts.Port,
		ReservedCpuAnnoKey:     opts.ReservedCpuAnnoKey,
		ReservedMemAnnoKey:     opts.ReservedMemAnnoKey,
		ConsiderUnboundLocalPV: opts.ConsiderUnboundLocalPV,
	}

	config := &config.Config{
		Options: options,

		Client:          kubeClient,
		InformerFactory: informerFactory,
	}

	return config, nil
}
