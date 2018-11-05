package config

import (
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
)

type Config struct {
	Options *OptionConfig

	Client          clientset.Interface
	InformerFactory informers.SharedInformerFactory
}

type OptionConfig struct {
	ListenPort           int
	ReservedCpuAnnoKey   string
	ReservedMemAnnoKey   string

	ConsiderUnboundLocalPV bool
}
