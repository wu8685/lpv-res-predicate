package predicate

import "k8s.io/api/core/v1"

type FailedNodesMap map[string]string

// fail to import following types from kubernetes originally
type ExtenderArgs struct {
	Pod *v1.Pod
	Nodes *v1.NodeList
	NodeNames []string
}

type ExtenderFilterResult struct {
	Nodes *v1.NodeList
	NodeNames *[]string
	FailedNodes FailedNodesMap
	Error string
}
