package predicate

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/wu8685/lpv-res-predicate/pkg"
	"github.com/wu8685/lpv-res-predicate/pkg/config"
	"github.com/wu8685/lpv-res-predicate/pkg/handlers"
)

var (
	EmptyFilterResult = &ExtenderFilterResult{
		NodeNames:   &[]string{},
		FailedNodes: map[string]string{},
	}

	EmptyPods = []*v1.Pod{}
)

func init() {
	pkg.RegisterHandlerInit("filter", NewPredicateHandler)
}

type PredicateHandler struct {
	cfg      *config.OptionConfig
	accessor resourceAccessor
}

func NewPredicateHandler(cfg *config.Config) (pkg.Handler, error) {
	handler := &PredicateHandler{
		cfg:      cfg.Options,
		accessor: NewResourceAccessor(cfg),
	}
	return handler, nil
}

func (h *PredicateHandler) RestInfos() []pkg.RestInfo {
	return []pkg.RestInfo{
		{
			"/lpvReservedResource",
			[]string{"POST"},
			h.handle,
		},
	}
}

func (h *PredicateHandler) handle(w http.ResponseWriter, r *http.Request) {
	body := &ExtenderArgs{}
	if err := handlers.ReadRequestBody(r, body); err != nil {
		glog.Errorf("Fail to read request body: %s", err)
		h.responseErr(w, err)
		return
	}

	nodeNames, nodeMap, hasNode := getNodeNames(body)
	result, err := h.predicate(nodeNames, body.Pod)
	if err != nil {
		glog.Errorf("Fail to predicate pod %s on nodes %v: %s", body.Pod.Name, nodeNames, err)
		h.responseErr(w, err)
	} else {
		glog.V(1).Infof("Predicate pod %s on nodes %v, fit nodes: %v", body.Pod.Name, nodeNames, result.NodeNames)
		if hasNode {
			nodeList := body.Nodes
			nodeList.Items = make([]v1.Node, len(*result.NodeNames))
			for i, nodeName := range *result.NodeNames {
				nodeList.Items[i] = *nodeMap[nodeName]
			}
			result.Nodes = nodeList
		}
		handlers.WriteResponse(w, 200, result)
	}
}

func (h *PredicateHandler) predicate(NodeNames []string, pod *v1.Pod) (*ExtenderFilterResult, error) {
	glog.V(0).Infof("Start predicating pod %s", pod.Name)

	if glog.V(1) {
		startTime := time.Now()
		defer func() {
			glog.V(1).Infof("Predicate pod %s cost %s", pod.Name, time.Now().Sub(startTime).String())
		}()
	}

	if len(NodeNames) == 0 {
		return EmptyFilterResult, nil
	}

	fitNodeNames := []string{}
	failedNodesMap := map[string]string{}
	for _, nodeName := range NodeNames {
		node, err := h.accessor.GetNode(nodeName)
		if err != nil {
			return nil, fmt.Errorf("fail to get node %s from informer: %s", nodeName, err)
		}

		if fit, reason, err := h.predicateOneNode(node, pod); err != nil {
			return nil, fmt.Errorf("fail to predicate pod %s on node %s: %s", pod.Name, nodeName, err)
		} else if fit {
			fitNodeNames = append(fitNodeNames, nodeName)
		} else {
			failedNodesMap[nodeName] = reason
		}
	}

	return &ExtenderFilterResult{
		NodeNames:   &fitNodeNames,
		FailedNodes: failedNodesMap,
	}, nil
}

func (h *PredicateHandler) predicateOneNode(node *v1.Node, pod *v1.Pod) (bool, string, error) {
	pvs, unboundPvs, err := h.getReservedResourceLocalPVOnNode(node)
	if err != nil {
		return false, "", err
	}

	podPVs, err := h.localPersistentVolumeOnPodOnNode(pod, pvs, unboundPvs)
	if err != nil {
		return false, "", err
	}

	// if the pod uses local PVs with reserved resource, predicate with the reserved resource on each local PV
	// or predicate with the node remained resources after reserving resources for all of the local PVs
	if len(podPVs) > 0 {
		for _, pv := range podPVs {
			remainedPVCpu, configedCpu, remainedPVMem, configedMem, canSkip, err := h.remainReservedResources(pv, node.Name)
			if canSkip {
				continue
			}

			if err != nil {
				return false, "", err
			}

			if configedCpu {
				if _, enough := subPodCpuRequest(remainedPVCpu, pod); !enough {
					return false, fmt.Sprintf("reserved cpu of local persistent volume %s is not enough", pv.Name), nil
				}
			}

			if configedMem {
				if _, enough := subPodMemRequest(remainedPVMem, pod); !enough {
					return false, fmt.Sprintf("reserved memory of local persistent volume %s is not enough", pv.Name), nil
				}
			}
		}
	} else {
		// consider all kinds of node resources
		availableNodeCpu, availableNodeMem, err := h.availableResource(node)
		if err != nil {
			return false, "", err
		}

		if h.cfg.ConsiderUnboundLocalPV {
			for name, unboundPv := range unboundPvs {
				pvs[name] = unboundPv
			}
		}

		for _, pv := range pvs {
			remainedPVCpu, configedCpu, remainedPVMem, configedMem, canSkip, err := h.remainReservedResources(pv, node.Name)
			if canSkip {
				continue
			}

			if err != nil {
				return false, "", err
			}

			if configedCpu {
				availableNodeCpu.Sub(remainedPVCpu)
			}
			if configedMem {
				availableNodeMem.Sub(remainedPVMem)
			}
		}

		if _, cpuEnough := subPodCpuRequest(*availableNodeCpu, pod); !cpuEnough {
			return false, "cpu not enough after reserving for local persistent volume", nil
		}
		if _, memEnough := subPodMemRequest(*availableNodeMem, pod); !memEnough {
			return false, "memory not enough after reserving for local persistent volume", nil
		}
	}
	return true, "", nil
}

func (h *PredicateHandler) getReservedResourceLocalPVOnNode(node *v1.Node) (map[string]*v1.PersistentVolume, map[string]*v1.PersistentVolume, error) {
	pvs, err := h.accessor.GetAllPersistentVolume()
	if err != nil {
		return nil, nil, err
	}

	pvsOnNode := map[string]*v1.PersistentVolume{}
	unboundPvsOnNode := map[string]*v1.PersistentVolume{}
	for _, pv := range pvs {
		// skip PVs with no reserved resources or PVs running out of reserved resources
		consider, bound := considerReserveResource(pv, h.cfg.ReservedCpuAnnoKey, h.cfg.ReservedMemAnnoKey)
		if !consider {
			continue
		}

		if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil || pv.Spec.NodeAffinity.Required.NodeSelectorTerms == nil {
			glog.V(0).Infof("Local persistent volume has reserved resource, but has malformed NodeAffinity")
			continue
		}
		if nodeMatchesNodeSelectorTerms(node, pv.Spec.NodeAffinity.Required.NodeSelectorTerms) {
			if bound {
				pvsOnNode[pv.Name] = pv
			} else {
				unboundPvsOnNode[pv.Name] = pv
			}
		}
	}
	return pvsOnNode, unboundPvsOnNode, nil
}

func (h *PredicateHandler) getPodsOnPVOnNode(pvName string, node string) ([]*v1.Pod, error) {
	pv, err := h.accessor.GetPersistentVolume(pvName)
	if err != nil {
		return EmptyPods, nil
	}

	// no pvc binding to this pv
	if pv.Status.Phase != v1.VolumeBound {
		return EmptyPods, nil
	}

	claimRef := pv.Spec.ClaimRef
	nsPods, err := h.accessor.GetPodsInNamespace(claimRef.Namespace)
	if err != nil {
		return EmptyPods, err
	}

	podsOnNode := []*v1.Pod{}
	for _, pod := range nsPods {
		if pod.Spec.NodeName == node {
			podsOnNode = append(podsOnNode, pod)
		}
	}
	return getMountPods(podsOnNode, claimRef.Name)
}

func (h *PredicateHandler) localPersistentVolumeOnPodOnNode(pod *v1.Pod,
					localPVOnNode map[string]*v1.PersistentVolume,
					unboundLocalPVOnNode map[string]*v1.PersistentVolume) (map[string]*v1.PersistentVolume, error) {
	volumes := getPvcVolumeSources(pod.Spec.Volumes)
	pvs := map[string]*v1.PersistentVolume{}
	unboundPVs := []*v1.PersistentVolume{}
	for _, unboundPV := range unboundLocalPVOnNode {
		unboundPVs = append(unboundPVs, unboundPV)
	}

	for _, v := range volumes {
		pvc, err := h.accessor.GetPersistentVolumeClaim(pod.Namespace, v.VolumeSource.PersistentVolumeClaim.ClaimName)
		if err != nil {
			return nil, err
		}

		// no need to check fully binding by annotation pv.kubernetes.io/bind-completed here
		if len(pvc.Spec.VolumeName) > 0 {
			// bound pvc
			if pv, exist := localPVOnNode[pvc.Spec.VolumeName]; exist {
				pvs[pv.Name] = pv
			}
		} else if h.cfg.ConsiderUnboundLocalPV {
			// unbound pvc
			unboundPVs, err := findMatchingVolume(pvc, unboundPVs,nil, nil,false)
			if err != nil {
				glog.Errorf("Fail to find unbound local pv for volume %s of pod %s", v.Name, pod.Name)
				continue
			}
			for _, unboundPV := range unboundPVs {
				pvs[unboundPV.Name] = unboundPV
			}
		}
	}
	return pvs, nil
}

// returns the available resource on the node
func (h *PredicateHandler) availableResource(node *v1.Node) (*resource.Quantity, *resource.Quantity, error) {
	pods, err := h.accessor.GetAllPods()
	if err != nil {
		return nil, nil, fmt.Errorf("fail to get pod when calculate availabel resource of node %s: %s", node.Name, err)
	}

	cpuResource := node.Status.Allocatable.Cpu().DeepCopy()
	memResource := node.Status.Allocatable.Memory().DeepCopy()
	for _, pod := range pods {
		if pod.Spec.NodeName != node.Name {
			continue
		}

		cpuResource, _ = subPodCpuRequest(cpuResource, pod)
		memResource, _ = subPodMemRequest(memResource, pod)
	}
	return &cpuResource, &memResource, nil
}

func (h *PredicateHandler) getReservedResource(pv *v1.PersistentVolume) (resource.Quantity, bool, resource.Quantity, bool, error) {
	reservedCpu, hasCpu, err := getResevedResource(pv, h.cfg.ReservedCpuAnnoKey)
	if err != nil {
		// malformat cpu value
		glog.V(0).Infof("Malformat reserved cpu annotation value %v: %s", pv.Annotations, err)
		return reservedCpu, false, ZeroQuantity, false, err
	}

	reservedMem, hasMem, err := getResevedResource(pv, h.cfg.ReservedMemAnnoKey)
	if err != nil {
		// malformat memory value
		glog.V(0).Infof("Malformat reserved memory annotation value %v: %s", pv.Annotations, err)
		return reservedCpu, false, reservedMem, false, err
	}

	return reservedCpu, hasCpu, reservedMem, hasMem, nil
}

// returns the remained resources after allocating some resources to binding pod
func (h *PredicateHandler) remainReservedResources(pv *v1.PersistentVolume, nodeName string) (resource.Quantity, bool, resource.Quantity, bool, bool, error) {
	reservedCpu, configedCpu, reservedMem, configedMem, err := h.getReservedResource(pv)
	if err != nil {
		// skip this pv if malformed reserved resource value
		return reservedCpu, configedCpu, reservedMem, configedMem, true, err
	}

	// It is possible that one pod binds multiple local persistent volume.
	// In this case, this pod will be considered by each pv to calculate left reserved resources,
	// so there will not be mistake
	pods, err := h.getPodsOnPVOnNode(pv.Name, nodeName)
	if err != nil {
		return reservedCpu, configedCpu, reservedMem, configedMem, false, err
	}

	cpuEnough := true
	memEnough := true
	if configedCpu {
		reservedCpu, cpuEnough = subCpuRequest(reservedCpu, pods)
	}

	if configedMem {
		reservedMem, memEnough = subMemRequest(reservedMem, pods)
	}

	// mark pv as running out of reserved resources
	if !cpuEnough || !memEnough {
		// if one of cpu and memory runs out, stop considering the reserved resource of this local pv
		return reservedCpu, configedCpu, reservedMem, configedMem, true, nil
	}

	return reservedCpu, configedCpu, reservedMem, configedMem, false, nil
}

func (h *PredicateHandler) responseErr(w http.ResponseWriter, err error) {
	result := &ExtenderFilterResult{}
	result.Error = err.Error()
	handlers.WriteResponse(w, 200, result)
}
