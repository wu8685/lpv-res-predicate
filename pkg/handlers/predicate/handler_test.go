package predicate

import (
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/wu8685/lpv-res-predicate/pkg/config"
)

const (
	CPU_KEY = "reserved-cpu"
	MEM_KEY = "reserved-mem"

	NODE_LABEL = "nodeName"
)

var (
	opts = &config.OptionConfig{
		ListenPort: 8080,
		ReservedCpuAnnoKey: CPU_KEY,
		ReservedMemAnnoKey: MEM_KEY,
		ConsiderUnboundLocalPV: false,
	}
)

func TestPredicateWithMultiNodes(t *testing.T) {
	accessor := &MockAccessor{}

	// node
	node1 := buildNode(t, "node1", "8", "10G")
	node2 := buildNode(t, "node2", "8", "10G")
	accessor.Nodes = []*v1.Node {
		node1, node2,
	}

	// pv
	cpu := "4"
	mem := "1G"
	pv := buildPV("pv1", &node1.Name, &cpu, &mem)
	accessor.PVs = []*v1.PersistentVolume {
		pv,
	}

	// pvc
	pvc := buildPVC("test", "pvc1")
	accessor.PVCs = map[string][]*v1.PersistentVolumeClaim {}
	accessor.PVCs[pvc.Namespace] = []*v1.PersistentVolumeClaim{ pvc }
	bind(pv, pvc)

	handler := &PredicateHandler{opts, accessor}

	pod1 := buildPod(t, "test", "pod1", []string{pvc.Name}, []string{"2"}, []string{"2G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod1))

	pod1 = buildPod(t, "test", "pod1", []string{pvc.Name}, []string{"5"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod1))

	pod1 = buildPod(t, "test", "pod1", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod1))

	// provision this pod
	bindNode(pod1, node1.Name)
	accessor.Pods = map[string][]*v1.Pod {
		pod1.Namespace: {
			pod1,
		},
	}

	// reserved resource is indifficient
	pod2 := buildPod(t, "test", "pod2", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod2))

	// node resource is difficient
	pod3 := buildPod(t, "test", "pod3", []string{}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod3))

	// pod migrates
	bindNode(pod1, "otherNode")
	pod4 := buildPod(t, "test", "pod4", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod4))
}

func TestPredicateWithMultiPodsOnDifferentNodes(t *testing.T) {
	accessor := &MockAccessor{}

	// node
	node1 := buildNode(t, "node1", "4", "10G")
	node2 := buildNode(t, "node2", "4", "10G")
	accessor.Nodes = []*v1.Node {
		node1, node2,
	}

	// pv
	cpu := "2"
	mem := "2G"
	pv := buildPV("pv1", &node1.Name, &cpu, &mem)
	accessor.PVs = []*v1.PersistentVolume {
		pv,
	}

	// pvc
	pvc := buildPVC("test", "pvc1")
	accessor.PVCs = map[string][]*v1.PersistentVolumeClaim {}
	accessor.PVCs[pvc.Namespace] = []*v1.PersistentVolumeClaim{ pvc }
	bind(pv, pvc)

	handler := &PredicateHandler{opts, accessor}

	pod1 := buildPod(t, "test", "pod1", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod1))

	// provision this pod
	bindNode(pod1, node1.Name)
	accessor.AddPod(pod1)

	pod2 := buildPod(t, "test", "pod2", []string{}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node2, pod2))

	// provision this pod
	bindNode(pod2, node2.Name)
	accessor.AddPod(pod2)

	pod3 := buildPod(t, "test", "pod3", []string{}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod3))

	pod4 := buildPod(t, "test", "pod4", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod4))
}

func TestPredicateWithUnboundPV(t *testing.T) {
	accessor := &MockAccessor{}

	// node
	node1 := buildNode(t, "node1", "4", "10G")
	node2 := buildNode(t, "node2", "4", "10G")
	accessor.Nodes = []*v1.Node {
		node1, node2,
	}

	// pv
	cpu := "2"
	mem := "2G"
	pv := buildPV("pv1", &node1.Name, &cpu, &mem)
	accessor.PVs = []*v1.PersistentVolume {
		pv,
	}

	// pvc
	pvc := buildPVC("test", "pvc1")
	accessor.PVCs = map[string][]*v1.PersistentVolumeClaim {}
	accessor.PVCs[pvc.Namespace] = []*v1.PersistentVolumeClaim{ pvc }

	opts.ConsiderUnboundLocalPV = true
	handler := &PredicateHandler{opts, accessor}

	pod1 := buildPod(t, "test", "pod1", []string{pvc.Name}, []string{"2"}, []string{"3G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod1))

	pod1 = buildPod(t, "test", "pod1", []string{}, []string{"2"}, []string{"8G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod1))

	pod1 = buildPod(t, "test", "pod1", []string{}, []string{"2"}, []string{"9G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod1))

	pod1 = buildPod(t, "test", "pod1", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod1))

	// provision this pod
	bindNode(pod1, node1.Name)
	accessor.AddPod(pod1)

	pod2 := buildPod(t, "test", "pod2", []string{}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node2, pod2))

	// provision this pod
	bindNode(pod2, node2.Name)
	accessor.AddPod(pod2)

	// pod1 takes the node resources and lpv reserves the rest cpu, so no cpu left
	pod3 := buildPod(t, "test", "pod3", []string{}, []string{"2"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod3))

	// It should be fit, because the pv is still Unbound and not able to find binding pods.
	// So the reserved resource is still unused.
	// The first provisioning pod like pod1 should have made this pv Bound by scheduler.
	// So the case should pass
	pod4 := buildPod(t, "test", "pod4", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod4))

	pod5 := buildPod(t, "test", "pod5", []string{pvc.Name}, []string{"3"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod5))

	// if pod1 binds pv1
	bind(pv, pvc)

	// now the pod1 will use resource from pv reserved resource not from node, so pod3 is fit
	pod3 = buildPod(t, "test", "pod3", []string{}, []string{"2"}, []string{"1G"})
	assert(t).Fit(handler.predicateOneNode(node1, pod3))

	// now pv reserved resource is taken by pod1, so reserved resource is not enough
	pod4 = buildPod(t, "test", "pod4", []string{pvc.Name}, []string{"2"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod4))

	// still not enough for reserved resource
	pod5 = buildPod(t, "test", "pod5", []string{pvc.Name}, []string{"3"}, []string{"1G"})
	assert(t).Unfit(handler.predicateOneNode(node1, pod5))
}

func buildNode(t *testing.T, name string, cpu, mem string) *v1.Node {
	node := &v1.Node{}
	node.Name = name

	cpuQ, err := resource.ParseQuantity(cpu)
	if err != nil {
		t.Fatal()
	}

	memQ, err := resource.ParseQuantity(mem)
	if err != nil {
		t.Fatal()
	}

	node.Status = v1.NodeStatus{}
	node.Status.Allocatable = v1.ResourceList{
		"cpu": cpuQ,
		"memory": memQ,
	}

	node.Labels = map[string]string{
		NODE_LABEL: name,
	}

	return node
}

func buildPV(name string, node *string, reserveCpu, reserveMem *string) *v1.PersistentVolume {
	pv := &v1.PersistentVolume{}
	pv.Name = name

	if reserveCpu != nil {
		if pv.Annotations == nil {
			pv.Annotations = map[string]string{}
		}
		pv.Annotations[CPU_KEY] = *reserveCpu
	}
	if reserveMem != nil {
		if pv.Annotations == nil {
			pv.Annotations = map[string]string{}
		}
		pv.Annotations[MEM_KEY] = *reserveMem
	}

	pv.Spec = v1.PersistentVolumeSpec{}
	if node != nil {
		pv.Spec.Local = &v1.LocalVolumeSource{
			Path: "/tmp",
		}
		pv.Spec.NodeAffinity = &v1.VolumeNodeAffinity{}
		pv.Spec.NodeAffinity.Required = &v1.NodeSelector{}
		pv.Spec.NodeAffinity.Required.NodeSelectorTerms = []v1.NodeSelectorTerm {
			{
				MatchExpressions: []v1.NodeSelectorRequirement{
					{
						Key: NODE_LABEL,
						Operator: v1.NodeSelectorOpIn,
						Values: []string{*node},
					},
				},
			},
		}
	}

	pv.Status = v1.PersistentVolumeStatus{}
	pv.Status.Phase = v1.VolumeAvailable

	return pv
}

func buildPVC(namespace, name string) *v1.PersistentVolumeClaim {
	pvc := &v1.PersistentVolumeClaim{}
	pvc.Name = name
	pvc.Namespace = namespace
	pvc.Status = v1.PersistentVolumeClaimStatus{}
	pvc.Status.Phase = v1.ClaimPending
	return pvc
}

func bind(pv *v1.PersistentVolume, pvc *v1.PersistentVolumeClaim) {
	pv.Spec.ClaimRef = &v1.ObjectReference{
		Name: pvc.Name,
		Namespace: pvc.Namespace,
	}
	pv.Status.Phase = v1.VolumeBound

	pvc.Spec.VolumeName = pv.Name
	pvc.Status.Phase = v1.ClaimBound
}

func buildPod(t *testing.T, namespace, name string, pvcs []string, cpu, mem []string) *v1.Pod {
	if len(cpu) != len(mem) {
		t.Fatal()
	}

	pod := &v1.Pod{}
	pod.Name = name
	pod.Namespace = namespace

	pod.Spec = v1.PodSpec{}
	pod.Status = v1.PodStatus{}

	pod.Spec.Volumes = []v1.Volume{}
	for _, pvc := range pvcs {
		v := v1.Volume{}
		v.VolumeSource.PersistentVolumeClaim = &v1.PersistentVolumeClaimVolumeSource{}
		v.VolumeSource.PersistentVolumeClaim.ClaimName = pvc
		pod.Spec.Volumes = append(pod.Spec.Volumes, v)
	}

	for i, cpu := range cpu {
		if pod.Spec.Containers == nil {
			pod.Spec.Containers = []v1.Container{}
		}

		c := v1.Container{}
		c.Resources = v1.ResourceRequirements{}
		c.Resources.Requests = v1.ResourceList{}
		cpuQ, err := resource.ParseQuantity(cpu)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		memQ, err := resource.ParseQuantity(mem[i])
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		c.Resources.Requests[v1.ResourceCPU] = cpuQ
		c.Resources.Requests[v1.ResourceMemory] = memQ

		pod.Spec.Containers = append(pod.Spec.Containers, c)
	}

	return pod
}

func bindNode(pod *v1.Pod, nodeName string) {
	pod.Spec.NodeName = nodeName
}

type MockAccessor struct {
	Pods map[string][]*v1.Pod
	PVs  []*v1.PersistentVolume
	PVCs map[string][]*v1.PersistentVolumeClaim
	Nodes []*v1.Node
}

func (a *MockAccessor) GetAllPods() ([]*v1.Pod, error) {
	pods := []*v1.Pod{}
	for _, podsInNS := range a.Pods {
		for _, pod := range podsInNS {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func (a *MockAccessor) GetPodsInNamespace(namespace string) ([]*v1.Pod, error) {
	return a.Pods[namespace], nil
}

func (a *MockAccessor) GetPersistentVolume(name string) (*v1.PersistentVolume, error) {
	for _, pv := range a.PVs {
		if pv.Name == name {
			return pv, nil
		}
	}
	return nil, nil
}

func (a *MockAccessor) GetAllPersistentVolume() ([]*v1.PersistentVolume, error) {
	pvs := []*v1.PersistentVolume{}
	for _, pv := range a.PVs {
		pvs = append(pvs, pv)
	}
	return pvs, nil
}

func (a *MockAccessor) GetPersistentVolumeClaim(namespace, name string) (*v1.PersistentVolumeClaim, error) {
	pvcInNS, exist := a.PVCs[namespace]
	if !exist {
		return nil, nil
	}

	for _, pvc := range pvcInNS {
		if pvc.Name == name {
			return pvc, nil
		}
	}
	return nil, nil
}

func (a *MockAccessor) GetNode(name string) (*v1.Node, error) {
	for _, node := range a.Nodes {
		if node.Name == name {
			return node, nil
		}
	}
	return nil, nil
}

func (a *MockAccessor) UpdatePersistentVolume(volume *v1.PersistentVolume) (*v1.PersistentVolume, error) {
	return volume, nil
}

func (a *MockAccessor) AddPod(pod *v1.Pod) {
	if a.Pods == nil {
		a.Pods = map[string][]*v1.Pod{}
	}

	if _, exist := a.Pods[pod.Namespace]; !exist {
		a.Pods[pod.Namespace] = []*v1.Pod{}
	}

	pods := a.Pods[pod.Namespace]
	pods = append(pods, pod)
	a.Pods[pod.Namespace] = pods
}

type asserter struct {
	t *testing.T
}

func assert(t *testing.T) *asserter {
	return &asserter{t}
}

func (a *asserter) Fit(fit bool, reason string, err error) {
	if err != nil {
		a.t.Fatalf("unexpected err: %s", err)
	}

	if ! fit {
		a.t.Fatalf("unexpected: %s", reason)
	}
}

func (a *asserter) Unfit(fit bool, reason string, err error) {
	if err != nil {
		a.t.Fatalf("unexpected err: %s", err)
	}

	if fit {
		a.t.Fatalf("unexpected fit")
	}
}