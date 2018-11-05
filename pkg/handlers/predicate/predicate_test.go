package predicate

import (
	"testing"

	"k8s.io/api/core/v1"
)

func TestGetNilMountPods(t *testing.T) {
	nilPods := []*v1.Pod{
		{},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatal("unexpecting panic")
		}
	}()

	_, err := getMountPods(nilPods, "test")
	if err != nil {
		t.Fatal()
	}
}

func TestGetMountPod(t *testing.T) {
	pvcName := "pvc"
	pods := []*v1.Pod{
		{
			Spec: v1.PodSpec{
				Volumes: []v1.Volume{
					{
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcName,
							},
						},
					},
				},
			},
		},
		{},
	}
	mountPods, err := getMountPods(pods, pvcName)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}
	if len(mountPods) != 1 {
		t.Fatalf("expected 1 mounted pod, got %d", len(mountPods))
	}
}

func TestGetPVOnNode(t *testing.T) {
	key := "kubernetes.io/hostname"
	value := "test"

	node := &v1.Node{}
	node.Labels = map[string]string{key: value}

	buildSelectorTerm := func(value string) *v1.NodeSelectorTerm {
		return &v1.NodeSelectorTerm{
			MatchExpressions: []v1.NodeSelectorRequirement{
				{
					Key:      key,
					Operator: v1.NodeSelectorOpIn,
					Values:   []string{value},
				},
			},
		}
	}

	selectorTerm := buildSelectorTerm(value)
	if !nodeMatchesNodeSelectorTerms(node, []v1.NodeSelectorTerm{*selectorTerm}) {
		t.Fatal("unexpected")
	}

	selectorTerm = buildSelectorTerm("NotMatch")
	if nodeMatchesNodeSelectorTerms(node, []v1.NodeSelectorTerm{*selectorTerm}) {
		t.Fatal("unexpected")
	}
}

func TestConsiderReserveResource(t *testing.T) {
	cpu := "reservedCpu"
	mem := "reservedMem"
	out := "outOfReserved"
	consider := func(pv *v1.PersistentVolume) bool {
		cosider, _ := considerReserveResource(pv, cpu, mem)
		return cosider
	}

	buildLocalPV := func(path *string, phase v1.PersistentVolumePhase) *v1.PersistentVolume {
		pv := &v1.PersistentVolume{}
		if path != nil {
			pv.Spec.Local = &v1.LocalVolumeSource{}
			pv.Spec.Local.Path = *path
		}

		pv.Status.Phase = phase
		return pv
	}

	pv := buildLocalPV(nil, v1.VolumeAvailable)
	if consider(pv) {
		t.Fatal("unexpected")
	}

	pv = buildLocalPV(nil, v1.VolumeBound)
	if consider(pv) {
		t.Fatal("unexpected")
	}

	path := "/tmp"
	pv = buildLocalPV(&path, v1.VolumeBound)
	if consider(pv) {
		t.Fatal("unexpected")
	}


	buildLocalPVWithAnno := func(annos map[string]string) *v1.PersistentVolume {
		pv := &v1.PersistentVolume{}
		pv.Status.Phase = v1.VolumeBound
		pv.Spec.Local = &v1.LocalVolumeSource{}
		pv.Spec.Local.Path = "/tmp"

		pv.Annotations = map[string]string{}
		for k, v := range annos {
			pv.Annotations[k] = v
		}

		return pv
	}

	trueCases := []map[string]string{
		{
			cpu: "1024",
			mem: "100G",
		},
		{
			cpu: "1024",
		},
		{
			mem: "100G",
		},
		{
			cpu: "-100",
			mem: "100G",
		},
		{
			cpu: "1024",
			mem: "100X",
		},
		{
			cpu: "1024",
			out: "other",
		},
		{
			// invalid value will be filtered out later
			cpu: "1024X",
		},
		{
			mem: "100X",
		},
	}
	for _, c := range trueCases {
		if !consider(buildLocalPVWithAnno(c)) {
			t.Fatalf("expected true: %v", c)
		}
	}

	falseCases := []map[string]string{
		{
			// no annotations
		},
		{
			"hello": "world",
		},
	}
	for _, c := range falseCases {
		if consider(buildLocalPVWithAnno(c)) {
			t.Fatalf("expected false: %v", c)
		}
	}
}
