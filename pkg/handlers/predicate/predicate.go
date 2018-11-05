package predicate

import (
	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/scheduler/algorithm"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	ZeroQuantity, _ = resource.ParseQuantity("0")
)

func getNodeNames(body *ExtenderArgs) ([]string, map[string]*v1.Node, bool) {
	if body == nil || body.Pod == nil || (body.NodeNames == nil && body.Nodes == nil) {
		return []string{}, nil, false
	}

	if body.NodeNames != nil {
		return body.NodeNames, nil, false
	}

	nodeNames := make([]string, len(body.Nodes.Items))
	nodeMap := map[string]*v1.Node{}
	for i, node := range body.Nodes.Items {
		nodeNames[i] = node.Name
		nodeMap[node.Name] = &node
	}

	return nodeNames, nodeMap, true
}

func getMountPods(nsPods []*v1.Pod, pvcName string) ([]*v1.Pod, error) {
	pods := []*v1.Pod{}

	for _, pod := range nsPods {
		pvcs := getPvcVolumeSources(pod.Spec.Volumes)
		if pvcs == nil {
			continue
		}

		for _, pvc := range pvcs {
			if pvc.PersistentVolumeClaim.ClaimName == pvcName {
				pods = append(pods, pod)
			}
		}
	}

	return pods, nil
}

func getPvcVolumeSources(volumes []v1.Volume) []v1.Volume {
	if volumes == nil {
		return nil
	}

	var pvcs []v1.Volume

	for _, volume := range volumes {
		if volume.VolumeSource.PersistentVolumeClaim != nil {
			pvcs = append(pvcs, volume)
		}
	}

	return pvcs
}

func nodeMatchesNodeSelectorTerms(node *v1.Node, nodeSelectorTerms []v1.NodeSelectorTerm) bool {
	nodeFields := map[string]string{}
	for k, f := range algorithm.NodeFieldSelectorKeys {
		nodeFields[k] = f(node)
	}
	return v1helper.MatchNodeSelectorTerms(nodeSelectorTerms, labels.Set(node.Labels), fields.Set(nodeFields))
}

func considerReserveResource(pv *v1.PersistentVolume, reservedCpuAnnoKey, reservedMemAnnoKey string) (bool, bool) {
	bound := pv.Status.Phase == v1.VolumeBound
	// skip no local PV and Unbound PV
	if pv.Spec.Local == nil {
		return false, bound
	}

	// skip local PV with no reserved resource annotation
	if pv.Annotations == nil || len(pv.Annotations) == 0 {
		return false, bound
	}
	_, cpuExist := pv.Annotations[reservedCpuAnnoKey];
	_, memExist := pv.Annotations[reservedMemAnnoKey];
	if !cpuExist && !memExist {
		return false, bound
	}

	return true, bound
}

func getResevedResource(pv *v1.PersistentVolume, key string) (resource.Quantity, bool, error) {
	val, exist := pv.Annotations[key]
	if !exist {
		return ZeroQuantity, exist, nil
	}

	quantity, err := resource.ParseQuantity(val)
	return quantity, exist, err
}

func subCpuRequest(from resource.Quantity, pods []*v1.Pod) (resource.Quantity, bool) {
	return substractPodsResource(from, pods, func(container *v1.Container) *resource.Quantity {
		return container.Resources.Requests.Cpu()
	})
}

func subMemRequest(from resource.Quantity, pods []*v1.Pod) (resource.Quantity, bool) {
	return substractPodsResource(from, pods, func(container *v1.Container) *resource.Quantity {
		return container.Resources.Requests.Memory()
	})
}

func subPodCpuRequest(from resource.Quantity, pod *v1.Pod) (resource.Quantity, bool) {
	return substractPodResource(from, pod, func(container *v1.Container) *resource.Quantity {
		return container.Resources.Requests.Cpu()
	})
}

func subPodMemRequest(from resource.Quantity, pod *v1.Pod) (resource.Quantity, bool) {
	return substractPodResource(from, pod, func(container *v1.Container) *resource.Quantity {
		return container.Resources.Requests.Memory()
	})
}

func substractPodsResource(from resource.Quantity, pods []*v1.Pod, getResource func(*v1.Container) *resource.Quantity) (resource.Quantity, bool) {
	enough := true
	for _, pod := range pods {
		from, enough = substractPodResource(from, pod, getResource)
		if !enough {
			return from, enough
		}
	}

	return from, true
}

func substractPodResource(from resource.Quantity, pod *v1.Pod, getResource func(*v1.Container) *resource.Quantity) (resource.Quantity, bool) {
	for _, container := range pod.Spec.Containers {
		from.Sub(*getResource(&container))
		if from.Cmp(ZeroQuantity) < 0 {
			return from, false
		}
	}

	return from, true
}