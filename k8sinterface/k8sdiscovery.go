package k8sinterface

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const ValueNotFound = -1

var ResourceGroupMapping = map[string]string{} // mapping of all supported Kubernetes cluster resources to apiVersion
var ResourceClusterScope = []string{}          // DEPRECATED - use the 'ResourceNamesapcedScope' instead
var ResourceNamesapcedScope = []string{}       // use this to determan if the resource is namespaced

// InitializeMapResources get supported api-resource (similar to 'kubectl api-resources') and map to 'ResourceGroupMapping' and 'ResourceNamesapcedScope'. If this function is not called, many functions may not work
func InitializeMapResources(discoveryClient discovery.DiscoveryInterface) {

	// resourceList, _ := discoveryClient.ServerPreferredResources()
	// if len(resourceList) != 0 {
	// 	setMapResources(resourceList)
	// }

	// set mock initialization (if resources where missing from discovery. this can happen when an error accurse while pulling the resources)
	InitializeMapResourcesMock()

}
func setMapResources(resourceList []*metav1.APIResourceList) {
	for i := range resourceList {
		if resourceList[i] == nil {
			continue
		}
		if len(resourceList[i].APIResources) == 0 {
			continue
		}

		// get group and version, we first split and then join for keeping our convention
		gv, err := schema.ParseGroupVersion(resourceList[i].GroupVersion)
		if err != nil {
			continue
		}

		// pre-defined resources to ignore
		if StringInSlice(ignoreGroups(), gv.Group) != ValueNotFound {
			continue
		}
		for _, apiResource := range resourceList[i].APIResources {
			if len(apiResource.Verbs) == 0 {
				continue
			}
			if _, ok := ResourceGroupMapping[apiResource.Name]; ok { // do not override resources in map
				continue
			}
			ResourceGroupMapping[apiResource.Name] = JoinGroupVersion(gv.Group, gv.Version)
			if apiResource.Namespaced {
				ResourceNamesapcedScope = append(ResourceNamesapcedScope, JoinResourceTriplets(gv.Group, gv.Version, apiResource.Name))
			} else { // DEPRECATED
				ResourceClusterScope = append(ResourceClusterScope, JoinResourceTriplets(gv.Group, gv.Version, apiResource.Name))

			}
		}
	}
}

// IsKindKubernetes check if the kind is known to be a kubernetes kind. In this check we do not test the apiVersion
func IsKindKubernetes(kind string) bool {
	if _, err := GetGroupVersionResource(kind); err == nil {
		return true
	}
	return false
}

// GetGroupVersionResource get the group and version from the resource name. Returns error if not found
func GetGroupVersionResource(resource string) (schema.GroupVersionResource, error) {
	resource = updateResourceKind(resource)
	if r, ok := ResourceGroupMapping[resource]; ok {
		gv := strings.Split(r, "/")
		if len(gv) >= 2 {
			return schema.GroupVersionResource{Group: gv[0], Version: gv[1], Resource: resource}, nil
		}
	}
	if resource == "" || resource == "*" {
		return schema.GroupVersionResource{}, nil
	}
	return schema.GroupVersionResource{}, fmt.Errorf("resource '%s' unknown. Make sure the resource is found at `kubectl api-resources`", resource)
}

// IsNamespaceScope returns true if the schema.GroupVersionResource is a kubernetes namespaced resource
func IsNamespaceScope(resource *schema.GroupVersionResource) bool {

	GetGroupVersionResource(resource.Resource)
	return StringInSlice(ResourceNamesapcedScope, GroupVersionResourceToString(resource)) != ValueNotFound
}

// IsResourceInNamespaceScope returns true if the resource is a kubernetes namespaced resource
func IsResourceInNamespaceScope(resource string) bool {
	gvr, err := GetGroupVersionResource(resource)
	if err != nil {
		return false
	}
	return IsNamespaceScope(&gvr)
}

// StringInSlice utility for finding a string in a slice. Returns ValueNotFound (-1) if the string is not found in the slice
func StringInSlice(strSlice []string, str string) int {
	for i := range strSlice {
		if strSlice[i] == str {
			return i
		}
	}
	return ValueNotFound
}

// JoinGroupVersion returns the group and version with the '/' separator
func JoinGroupVersion(group, version string) string {
	return fmt.Sprintf("%s/%s", group, version)
}

// SplitApiVersion receives apiVersion ("group/version") returns the group and version splitted
func SplitApiVersion(apiVersion string) (string, string) {
	group, version := "", ""
	p := strings.Split(apiVersion, "/")
	if len(p) >= 1 {
		group = p[0]
	}
	if len(p) >= 2 {
		version = p[1]
	}
	return group, version
}

// JoinResourceTriplets returns the group, version and kind with the '/' separator
func JoinResourceTriplets(group, version, resource string) string {
	return fmt.Sprintf("%s/%s/%s", group, version, resource)
}

// JoinResourceTriplets converts the schema.GroupVersionResource object to string by returning the group, version and kind with the '/' separator
func GroupVersionResourceToString(resource *schema.GroupVersionResource) string {
	return JoinResourceTriplets(resource.Group, resource.Version, resource.Resource)
}

// getResourceTriplets receives a partly defined schema.GroupVersionResource and returns a list of all resources (kinds) in the representation of group/version/resource that support what was missing
/*
Examples:

GetResourceTriplets("","","pods") -> []string{"/v1/pods"}
GetResourceTriplets("apps","v1","") -> []string{"apps/v1/deployments", "apps/v1/replicasets", ... }

*/
func getResourceTriplets(group, version, resource string) []string {
	resourceTriplets := []string{}
	if resource == "" {
		// load full map
		for k, v := range ResourceGroupMapping {
			if g := strings.Split(v, "/"); len(g) >= 2 {
				resourceTriplets = append(resourceTriplets, JoinResourceTriplets(g[0], g[1], k))
			}
		}
	} else if version == "" {
		// load by resource
		if v, ok := ResourceGroupMapping[resource]; ok {
			g := strings.Split(v, "/")
			if len(g) >= 2 {
				if group == "" {
					group = g[0]
				}
				resourceTriplets = append(resourceTriplets, JoinResourceTriplets(group, g[1], resource))
			}
		} else {
			// glog.Errorf("Resource '%s' unknown", resource)
		}
	} else if group == "" {
		// load by resource and version
		if v, ok := ResourceGroupMapping[resource]; ok {
			if g := strings.Split(v, "/"); len(g) >= 1 {
				resourceTriplets = append(resourceTriplets, JoinResourceTriplets(g[0], version, resource))
			}
		} else {
			// glog.Errorf("Resource '%s' unknown", resource)
		}
	} else {
		resourceTriplets = append(resourceTriplets, JoinResourceTriplets(group, version, resource))
	}
	return resourceTriplets
}

// DEPRECATED
func ResourceGroupToString(group, version, resource string) []string {
	return ResourceGroupToSlice(group, version, resource)
}

// ResourceGroupToSlice receives a partly defined schema.GroupVersionResource and returns a list of all resources (kinds) in the representation of group/version/resource that support what was missing. Will ignore if kind is not Kubernetes
/*
Examples:

GetResourceTriplets("*","*","pods") -> []string{"/v1/pods"}
GetResourceTriplets("apps","v1","*") -> []string{"apps/v1/deployments", "apps/v1/replicasets", ... }

*/
func ResourceGroupToSlice(group, version, resource string) []string {

	if group == "*" {
		group = ""
	}
	if version == "*" {
		version = ""
	}
	if resource == "*" {
		resource = ""
	}

	// if the resource is not kubernetes, do not edit or look for the group/version/kind in map
	if !IsKindKubernetes(resource) {
		return []string{JoinResourceTriplets(group, version, resource)}
	}
	resource = updateResourceKind(resource)
	return getResourceTriplets(group, version, resource)
}

// StringToResourceGroup convert a representation to the original triplet
/*
Examples:

StringToResourceGroup("apps/v1/deployments") -> "apps", "v1", "deployments"
StringToResourceGroup("/v1/pods") -> "", "v1", "pods"
*/
func StringToResourceGroup(str string) (string, string, string) {
	splitted := strings.Split(str, "/")
	for i := range splitted {
		if splitted[i] == "*" {
			splitted[i] = ""
		}
	}
	if len(splitted) == 3 {
		return splitted[0], splitted[1], splitted[2]
	}
	return "", "", ""
}

// updateResourceKind update kind from single to parallel
func updateResourceKind(resource string) string {
	resource = strings.ToLower(resource)

	if resource != "" && !strings.HasSuffix(resource, "s") {
		if strings.HasSuffix(resource, "y") {
			return fmt.Sprintf("%sies", strings.TrimSuffix(resource, "y")) // e.g. NetworkPolicy -> networkpolicies
		} else {
			return fmt.Sprintf("%ss", resource) // add 's' at the end of a resource
		}
	}
	return resource

}

func ignoreGroups() []string {
	return []string{"metrics.k8s.io"}
}

// TODO - consider using a k8s manifest validator
// Return if this object is a valide k8s workload
func IsTypeWorkload(object map[string]interface{}) bool {
	if object == nil {
		return false
	}
	// TODO - check if found in supported objects
	if _, ok := object["apiVersion"]; !ok {
		return false
	}
	if kind, ok := object["kind"]; ok {
		if k, ok := kind.(string); ok {
			if IsKindKubernetes(k) {
				return true
			}
		}
	}
	return false
}
