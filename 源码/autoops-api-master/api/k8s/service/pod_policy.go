package service

import "strings"

var protectedPodNamespaces = map[string]struct{}{
	"kube-system":     {},
	"kube-public":     {},
	"kube-node-lease": {},
	"calico-system":   {},
	"tigera-operator": {},
}

var protectedPodNamePrefixes = []string{
	"kube-",
	"calico-",
	"coredns",
	"metrics-server",
	"node-local-dns",
	"kubernetes-dashboard",
}

func isProtectedPodOperationTarget(namespaceName, podName string) bool {
	if _, protected := protectedPodNamespaces[strings.TrimSpace(namespaceName)]; protected {
		return true
	}

	podName = strings.TrimSpace(podName)
	for _, prefix := range protectedPodNamePrefixes {
		if strings.HasPrefix(podName, prefix) {
			return true
		}
	}
	return false
}
