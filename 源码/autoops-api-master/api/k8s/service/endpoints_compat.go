package service

import (
	"context"
	"fmt"

	"dodevops-api/api/k8s/model"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type serviceEndpointSummary struct {
	Ready    int
	NotReady int
	PodNames []string
}

// GetServiceEndpointSummary returns endpoint counts for a Service without
// forcing new clusters to read the deprecated core/v1 Endpoints API.
func GetServiceEndpointSummary(ctx context.Context, clientset *kubernetes.Clientset, namespace, serviceName string) (serviceEndpointSummary, error) {
	endpoints, err := listServiceEndpointsCompat(ctx, clientset, namespace, []string{serviceName})
	if err != nil {
		return serviceEndpointSummary{}, err
	}

	summary := serviceEndpointSummary{}
	seenPods := map[string]struct{}{}
	for _, endpoint := range endpoints[serviceName] {
		if endpoint.Ready {
			summary.Ready++
		} else {
			summary.NotReady++
		}
		if endpoint.PodName != "" {
			if _, ok := seenPods[endpoint.PodName]; !ok {
				summary.PodNames = append(summary.PodNames, endpoint.PodName)
				seenPods[endpoint.PodName] = struct{}{}
			}
		}
	}

	return summary, nil
}

func listServiceEndpointsCompat(ctx context.Context, clientset *kubernetes.Clientset, namespace string, serviceNames []string) (map[string][]model.ServiceEndpoint, error) {
	if len(serviceNames) == 0 {
		return map[string][]model.ServiceEndpoint{}, nil
	}

	endpointIndex, err := listEndpointSlicesForServices(ctx, clientset, namespace, serviceNames)
	if err == nil {
		return endpointIndex, nil
	}

	fallbackIndex, fallbackErr := listCoreEndpointsForServices(ctx, clientset, namespace, serviceNames)
	if fallbackErr == nil {
		return fallbackIndex, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, fallbackErr
}

func listAllServiceEndpointsCompat(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (map[string][]model.ServiceEndpoint, error) {
	endpointIndex, err := listAllEndpointSlices(ctx, clientset, namespace)
	if err == nil {
		return endpointIndex, nil
	}

	fallbackIndex, fallbackErr := listAllCoreEndpoints(ctx, clientset, namespace)
	if fallbackErr == nil {
		return fallbackIndex, nil
	}
	return nil, err
}

func listEndpointSlicesForServices(ctx context.Context, clientset *kubernetes.Clientset, namespace string, serviceNames []string) (map[string][]model.ServiceEndpoint, error) {
	endpointIndex := make(map[string][]model.ServiceEndpoint, len(serviceNames))
	for _, serviceName := range serviceNames {
		slices, err := clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, serviceName),
		})
		if err != nil {
			return nil, err
		}
		addEndpointSlicesToIndex(endpointIndex, slices)
	}
	return endpointIndex, nil
}

func listAllEndpointSlices(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (map[string][]model.ServiceEndpoint, error) {
	slices, err := clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	endpointIndex := make(map[string][]model.ServiceEndpoint)
	addEndpointSlicesToIndex(endpointIndex, slices)
	return endpointIndex, nil
}

func addEndpointSlicesToIndex(endpointIndex map[string][]model.ServiceEndpoint, slices *discoveryv1.EndpointSliceList) {
	if slices == nil {
		return
	}

	for i := range slices.Items {
		slice := &slices.Items[i]
		serviceName := slice.Labels[discoveryv1.LabelServiceName]
		if serviceName == "" {
			continue
		}

		ports := convertDiscoveryEndpointPorts(slice.Ports)
		for _, endpoint := range slice.Endpoints {
			ready := endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready
			for _, address := range endpoint.Addresses {
				item := model.ServiceEndpoint{
					IP:    address,
					Ready: ready,
					Ports: ports,
				}
				if endpoint.Hostname != nil {
					item.Hostname = *endpoint.Hostname
				}
				if endpoint.NodeName != nil {
					item.NodeName = *endpoint.NodeName
				}
				if endpoint.TargetRef != nil && endpoint.TargetRef.Kind == "Pod" {
					item.PodName = endpoint.TargetRef.Name
				}
				endpointIndex[serviceName] = append(endpointIndex[serviceName], item)
			}
		}
	}
}

func convertDiscoveryEndpointPorts(ports []discoveryv1.EndpointPort) []model.EndpointPort {
	endpointPorts := make([]model.EndpointPort, 0, len(ports))
	for _, port := range ports {
		item := model.EndpointPort{}
		if port.Name != nil {
			item.Name = *port.Name
		}
		if port.Port != nil {
			item.Port = *port.Port
		}
		if port.Protocol != nil {
			item.Protocol = string(*port.Protocol)
		}
		endpointPorts = append(endpointPorts, item)
	}
	return endpointPorts
}

func listCoreEndpointsForServices(ctx context.Context, clientset *kubernetes.Clientset, namespace string, serviceNames []string) (map[string][]model.ServiceEndpoint, error) {
	endpointIndex := make(map[string][]model.ServiceEndpoint, len(serviceNames))
	for _, serviceName := range serviceNames {
		endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		addCoreEndpointsToIndex(endpointIndex, endpoints)
	}
	return endpointIndex, nil
}

func listAllCoreEndpoints(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (map[string][]model.ServiceEndpoint, error) {
	endpoints, err := clientset.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	endpointIndex := make(map[string][]model.ServiceEndpoint)
	for i := range endpoints.Items {
		addCoreEndpointsToIndex(endpointIndex, &endpoints.Items[i])
	}
	return endpointIndex, nil
}

func addCoreEndpointsToIndex(endpointIndex map[string][]model.ServiceEndpoint, endpoints *corev1.Endpoints) {
	if endpoints == nil {
		return
	}

	for _, subset := range endpoints.Subsets {
		ports := convertCoreEndpointPorts(subset.Ports)
		for _, addr := range subset.Addresses {
			endpointIndex[endpoints.Name] = append(endpointIndex[endpoints.Name], convertCoreEndpointAddress(addr, true, ports))
		}
		for _, addr := range subset.NotReadyAddresses {
			endpointIndex[endpoints.Name] = append(endpointIndex[endpoints.Name], convertCoreEndpointAddress(addr, false, ports))
		}
	}
}

func convertCoreEndpointAddress(addr corev1.EndpointAddress, ready bool, ports []model.EndpointPort) model.ServiceEndpoint {
	endpoint := model.ServiceEndpoint{
		IP:       addr.IP,
		Hostname: addr.Hostname,
		Ready:    ready,
		Ports:    ports,
	}
	if addr.NodeName != nil {
		endpoint.NodeName = *addr.NodeName
	}
	if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
		endpoint.PodName = addr.TargetRef.Name
	}
	return endpoint
}

func convertCoreEndpointPorts(ports []corev1.EndpointPort) []model.EndpointPort {
	endpointPorts := make([]model.EndpointPort, 0, len(ports))
	for _, port := range ports {
		endpointPorts = append(endpointPorts, model.EndpointPort{
			Name:     port.Name,
			Port:     port.Port,
			Protocol: string(port.Protocol),
		})
	}
	return endpointPorts
}
