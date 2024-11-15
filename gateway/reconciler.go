package gateway

import (
	"fmt"
	"io"
	"log"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// Start watches all services and their pods
func (g *Gateway) Start() {
	go g.watchServices()
	go g.watchPods()
}

// watchServices watches for changes to services
func (g *Gateway) watchServices() {
	listWatch := cache.NewListWatchFromClient(
		g.clientset.CoreV1().RESTClient(),
		"services",
		"",
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		listWatch,
		&corev1.Service{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    g.handleServiceUpdate,
			UpdateFunc: func(oldObj, newObj interface{}) { g.handleServiceUpdate(newObj) },
			DeleteFunc: g.handleServiceDelete,
		},
	)

	log.Println("Starting Service watcher...")
	controller.Run(make(chan struct{}))
}

// watchPods watches for changes to pods
func (g *Gateway) watchPods() {
	listWatch := cache.NewListWatchFromClient(
		g.clientset.CoreV1().RESTClient(),
		"pods",
		"",
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		listWatch,
		&corev1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    g.reconcilePod,
			UpdateFunc: func(oldObj, newObj interface{}) { g.reconcilePod(newObj) },
			DeleteFunc: g.removePod,
		},
	)

	log.Println("Starting Pod watcher...")
	controller.Run(make(chan struct{}))
}

// handleServiceUpdate handles new or updated services
func (g *Gateway) handleServiceUpdate(obj interface{}) {
	service := obj.(*corev1.Service)

	labelSelector := labels.Set(service.Spec.Selector).AsSelector().String()
	log.Printf("Service %s/%s updated with selector: %s", service.Namespace, service.Name, labelSelector)
}

// handleServiceDelete removes a service and its pods from tracking
func (g *Gateway) handleServiceDelete(obj interface{}) {
	service := obj.(*corev1.Service)

	serviceKey := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
	g.servicePodsMu.Lock()
	defer g.servicePodsMu.Unlock()

	delete(g.servicePods, serviceKey)
	delete(g.roundRobinIndex, serviceKey)
	log.Printf("Service %s deleted", serviceKey)
}

// reconcilePod reconciles a pod and assigns it to the appropriate service
func (g *Gateway) reconcilePod(obj interface{}) {
	pod := obj.(*corev1.Pod)

	// Only consider running pods
	if pod.Status.Phase != corev1.PodRunning {
		return
	}

	g.servicePodsMu.Lock()
	defer g.servicePodsMu.Unlock()

	for serviceKey, endpoints := range g.servicePods {
		labelSelector := labels.SelectorFromSet(pod.Labels)
		if labelSelector.Matches(labels.Set(pod.Labels)) {
			endpoint := fmt.Sprintf("%s:%d", pod.Status.PodIP, 80) // Assuming HTTP on port 80
			for _, ep := range endpoints {
				if ep == endpoint {
					// Pod already exists for this service
					return
				}
			}
			g.servicePods[serviceKey] = append(g.servicePods[serviceKey], endpoint)
			log.Printf("Reconciled pod: %s for service: %s", pod.Name, serviceKey)
		}
	}
}

// removePod removes a pod from its associated service
func (g *Gateway) removePod(obj interface{}) {
	pod := obj.(*corev1.Pod)

	g.servicePodsMu.Lock()
	defer g.servicePodsMu.Unlock()

	for serviceKey, endpoints := range g.servicePods {
		endpoint := fmt.Sprintf("%s:%d", pod.Status.PodIP, 80)
		for i, ep := range endpoints {
			if ep == endpoint {
				// Remove the pod from the service
				g.servicePods[serviceKey] = append(endpoints[:i], endpoints[i+1:]...)
				log.Printf("Removed pod: %s from service: %s", pod.Name, serviceKey)
				break
			}
		}
	}
}

// GetNextPodEndpoint selects the next pod for a service in round-robin manner
func (g *Gateway) GetNextPodEndpoint(serviceKey string) (string, error) {
	g.servicePodsMu.Lock()
	defer g.servicePodsMu.Unlock()

	endpoints, exists := g.servicePods[serviceKey]
	if !exists || len(endpoints) == 0 {
		return "", fmt.Errorf("no pods available for service: %s", serviceKey)
	}

	index := g.roundRobinIndex[serviceKey]
	endpoint := endpoints[index]
	g.roundRobinIndex[serviceKey] = (index + 1) % len(endpoints)
	return endpoint, nil
}

// startHTTPServer starts the HTTP server for traffic routing
func (g *Gateway) startHTTPServer(port int) {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		service := req.URL.Query().Get("service")
		if service == "" {
			http.Error(w, "Missing service parameter", http.StatusBadRequest)
			return
		}

		endpoint, err := g.GetNextPodEndpoint(service)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusServiceUnavailable)
			return
		}

		proxyURL := fmt.Sprintf("http://%s%s", endpoint, req.URL.Path)
		log.Printf("Forwarding request to service %s -> %s", service, proxyURL)

		resp, err := http.Get(proxyURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error forwarding to pod: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	log.Printf("Starting HTTP server on port %d...", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
