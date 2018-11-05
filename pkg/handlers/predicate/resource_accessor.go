package predicate

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	listerv1 "k8s.io/client-go/listers/core/v1"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
)

type NoExist struct {
}

func (ne *NoExist) Error() string {
	return "not exist"
}

type resourceAccessor interface {
	GetAllPods() ([]*v1.Pod, error)
	GetPodsInNamespace(namespace string) ([]*v1.Pod, error)
	GetPersistentVolume(name string) (*v1.PersistentVolume, error)
	GetAllPersistentVolume() ([]*v1.PersistentVolume, error)
	GetPersistentVolumeClaim(namespace, name string) (*v1.PersistentVolumeClaim, error)
	GetNode(name string) (*v1.Node, error)

	UpdatePersistentVolume(volume *v1.PersistentVolume) (*v1.PersistentVolume, error)
}

type ResourceAccessor struct {
	cfg        *config.Config
	nodeLister listerv1.NodeLister
	pvLister   listerv1.PersistentVolumeLister
	pvcLister  listerv1.PersistentVolumeClaimLister
	podLister  listerv1.PodLister
}

func NewResourceAccessor(cfg *config.Config) *ResourceAccessor {
	accessor := &ResourceAccessor{
		cfg:        cfg,
		nodeLister: cfg.InformerFactory.Core().V1().Nodes().Lister(),
		pvLister:   cfg.InformerFactory.Core().V1().PersistentVolumes().Lister(),
		pvcLister:  cfg.InformerFactory.Core().V1().PersistentVolumeClaims().Lister(),
		podLister:  cfg.InformerFactory.Core().V1().Pods().Lister(),
	}

	return accessor
}

func (a *ResourceAccessor) GetAllPods() ([]*v1.Pod, error) {
	pods, err := a.podLister.List(labels.Everything())
	if err != nil {
		return pods, err
	}

	if pods == nil {
		return pods, &NoExist{}
	}

	return pods, err
}

func (a *ResourceAccessor) GetPodsInNamespace(namespace string) ([]*v1.Pod, error) {
	pod, err := a.podLister.Pods(namespace).List(labels.Everything())
	if err != nil {
		return pod, err
	}

	if pod == nil {
		return pod, &NoExist{}
	}

	return pod, err
}

func (a *ResourceAccessor) GetPersistentVolume(name string) (*v1.PersistentVolume, error) {
	pv, err := a.pvLister.Get(name)
	if err != nil {
		return nil, err
	}

	if pv == nil {
		return pv, &NoExist{}
	}

	return pv, nil
}

func (a *ResourceAccessor) GetAllPersistentVolume() ([]*v1.PersistentVolume, error) {
	pvs, err := a.pvLister.List(labels.Everything())
	if err != nil {
		return pvs, err
	}

	if pvs == nil {
		return pvs, &NoExist{}
	}

	return pvs, nil
}

func (a *ResourceAccessor) GetPersistentVolumeClaim(namespace, name string) (*v1.PersistentVolumeClaim, error) {
	pvc, err := a.pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err != nil {
		return pvc, err
	}

	if pvc == nil {
		return nil, &NoExist{}
	}

	return pvc, err
}

func (a *ResourceAccessor) GetNode(name string) (*v1.Node, error) {
	node, err := a.nodeLister.Get(name)
	if err != nil {
		return node, err
	}

	if node == nil {
		return node, &NoExist{}
	}

	return node, err
}

func (a *ResourceAccessor) UpdatePersistentVolume(volume *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	pv, err := a.cfg.Client.CoreV1().PersistentVolumes().Update(volume)
	if err != nil {
		return pv, err
	}

	if pv == nil {
		return pv, &NoExist{}
	}

	return pv, err
}
