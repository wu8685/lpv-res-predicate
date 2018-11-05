package predicate

import (
	"fmt"

	"k8s.io/kubernetes/pkg/features"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

// edit from kubernetes

// findMatchingVolume goes through the list of volumes to find the best matching volume
// for the claim.
//
// This function is used by both the PV controller and scheduler.
//
// delayBinding is true only in the PV controller path.  When set, prebound PVs are still returned
// as a match for the claim, but unbound PVs are skipped.
//
// node is set only in the scheduler path. When set, the PV node affinity is checked against
// the node's labels.
//
// excludedVolumes is only used in the scheduler path, and is needed for evaluating multiple
// unbound PVCs for a single Pod at one time.  As each PVC finds a matching PV, the chosen
// PV needs to be excluded from future matching.
func findMatchingVolume(
	claim *v1.PersistentVolumeClaim,
	volumes []*v1.PersistentVolume,
	node *v1.Node,
	excludedVolumes map[string]*v1.PersistentVolume,
	delayBinding bool) ([]*v1.PersistentVolume, error) {

	result := []*v1.PersistentVolume{}

	requestedQty := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestedClass := v1helper.GetPersistentVolumeClaimClass(claim)

	var selector labels.Selector
	if claim.Spec.Selector != nil {
		internalSelector, err := metav1.LabelSelectorAsSelector(claim.Spec.Selector)
		if err != nil {
			// should be unreachable code due to validation
			return nil, fmt.Errorf("error creating internal label selector for claim: %v: %v", claimToClaimKey(claim), err)
		}
		selector = internalSelector
	}

	// Go through all available volumes with two goals:
	// - find a volume that is either pre-bound by user or dynamically
	//   provisioned for this claim. Because of this we need to loop through
	//   all volumes.
	// - find the smallest matching one if there is no volume pre-bound to
	//   the claim.
	for _, volume := range volumes {
		if _, ok := excludedVolumes[volume.Name]; ok {
			// Skip volumes in the excluded list
			continue
		}

		volumeQty := volume.Spec.Capacity[v1.ResourceStorage]

		// check if volumeModes do not match (Alpha and feature gate protected)
		isMisMatch, err := checkVolumeModeMisMatches(&claim.Spec, &volume.Spec)
		if err != nil {
			return nil, fmt.Errorf("error checking if volumeMode was a mismatch: %v", err)
		}
		// filter out mismatching volumeModes
		if isMisMatch {
			continue
		}

		// check if PV's DeletionTimeStamp is set, if so, skip this volume.
		if utilfeature.DefaultFeatureGate.Enabled(features.StorageObjectInUseProtection) {
			if volume.ObjectMeta.DeletionTimestamp != nil {
				continue
			}
		}

		nodeAffinityValid := true
		if node != nil {
			// Scheduler path, check that the PV NodeAffinity
			// is satisfied by the node
			err := volumeutil.CheckNodeAffinity(volume, node.Labels)
			if err != nil {
				nodeAffinityValid = false
			}
		}

		if isVolumeBoundToClaim(volume, claim) {
			// this claim and volume are pre-bound; return
			// the volume if the size request is satisfied,
			// otherwise continue searching for a match
			if volumeQty.Cmp(requestedQty) < 0 {
				continue
			}

			// If PV node affinity is invalid, return no match.
			// This means the prebound PV (and therefore PVC)
			// is not suitable for this node.
			if !nodeAffinityValid {
				return nil, nil
			}

			// find one
			result = append(result, volume)
			continue
		}

		if node == nil && delayBinding {
			// PV controller does not bind this claim.
			// Scheduler will handle binding unbound volumes
			// Scheduler path will have node != nil
			continue
		}

		// filter out:
		// - volumes bound to another claim
		// - volumes whose labels don't match the claim's selector, if specified
		// - volumes in Class that is not requested
		// - volumes whose NodeAffinity does not match the node
		if volume.Spec.ClaimRef != nil {
			continue
		} else if selector != nil && !selector.Matches(labels.Set(volume.Labels)) {
			continue
		}
		if v1helper.GetPersistentVolumeClass(volume) != requestedClass {
			continue
		}
		if !nodeAffinityValid {
			continue
		}

		//if node != nil {
		//	// Scheduler path
		//	// Check that the access modes match
		//	if !checkAccessModes(claim, volume) {
		//		continue
		//	}
		//}
		// CHANGED: always check for local PV
		if !checkAccessModes(claim, volume) {
			continue
		}

		// find one
		result = append(result, volume)
	}

	return result, nil
}

func claimToClaimKey(claim *v1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s/%s", claim.Namespace, claim.Name)
}

// checkVolumeModeMatches is a convenience method that checks volumeMode for PersistentVolume
// and PersistentVolumeClaims along with making sure that the Alpha feature gate BlockVolume is
// enabled.
// This is Alpha and could change in the future.
func checkVolumeModeMisMatches(pvcSpec *v1.PersistentVolumeClaimSpec, pvSpec *v1.PersistentVolumeSpec) (bool, error) {
	if utilfeature.DefaultFeatureGate.Enabled(features.BlockVolume) {
		if pvSpec.VolumeMode != nil && pvcSpec.VolumeMode != nil {
			requestedVolumeMode := *pvcSpec.VolumeMode
			pvVolumeMode := *pvSpec.VolumeMode
			return requestedVolumeMode != pvVolumeMode, nil
		} else {
			// This also should retrun an error, this means that
			// the defaulting has failed.
			return true, fmt.Errorf("api defaulting for volumeMode failed")
		}
	} else {
		// feature gate is disabled
		return false, nil
	}
}

// Returns true if PV satisfies all the PVC's requested AccessModes
func checkAccessModes(claim *v1.PersistentVolumeClaim, volume *v1.PersistentVolume) bool {
	pvModesMap := map[v1.PersistentVolumeAccessMode]bool{}
	for _, mode := range volume.Spec.AccessModes {
		pvModesMap[mode] = true
	}

	for _, mode := range claim.Spec.AccessModes {
		_, ok := pvModesMap[mode]
		if !ok {
			return false
		}
	}
	return true
}

// isVolumeBoundToClaim returns true, if given volume is pre-bound or bound
// to specific claim. Both claim.Name and claim.Namespace must be equal.
// If claim.UID is present in volume.Spec.ClaimRef, it must be equal too.
func isVolumeBoundToClaim(volume *v1.PersistentVolume, claim *v1.PersistentVolumeClaim) bool {
	if volume.Spec.ClaimRef == nil {
		return false
	}
	if claim.Name != volume.Spec.ClaimRef.Name || claim.Namespace != volume.Spec.ClaimRef.Namespace {
		return false
	}
	if volume.Spec.ClaimRef.UID != "" && claim.UID != volume.Spec.ClaimRef.UID {
		return false
	}
	return true
}