package reconciler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// ServicePods stores pod endpoints and round-robin index for a service
type ServicePods struct {
	PodEndpoints  []string
	RoundRobinIdx int
}

// ClusterReconciler monitors all services and their associated pods
type ClusterReconciler struct {
	clientset     *kubernetes.Clientset
	servicePods   map[string]*ServicePods
	servicePodsMu sync.Mutex
}

// NewClusterReconciler creates a new reconciler for the cluster
func NewClusterReconciler() (*ClusterReconciler, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return &ClusterReconciler{
		clientset:   clientset,
		servicePods: make(map[string]*ServicePods),
	}, nil
}

// Start starts the reconciler
func (r *ClusterReconciler) Start() {
	go r.watchServices()
	go r.watchPods()
}

// watchServices watches for changes to services in the cluster
func (r *ClusterReconciler) watchServices() {
	listWatch := cache.NewListWatchFromClient(
		r.clientset.CoreV1().RESTClient(),
		"services",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		listWatch,
		&corev1.Service{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    r.addService,
			UpdateFunc: func(oldObj, newObj interface{}) { r.addService(newObj) },
			DeleteFunc: r.removeService,
		},
	)

	log.Println("Starting Service watcher...")
	controller.Run(make(chan struct{}))
}

// addService adds a new service to the reconciler
func (r *ClusterReconciler) addService(obj interface{}) {
	service := obj.(*corev1.Service)
	serviceKey := fmt.Sprintf("%s/%s", service.Namespace, service.Name)

	r.servicePodsMu.Lock()
	defer r.servicePodsMu.Unlock()

	if _, exists := r.servicePods[serviceKey]; !exists {
		r.servicePods[serviceKey] = &ServicePods{PodEndpoints: []string{}, RoundRobinIdx: 0}
		log.Printf("Added service: %s", serviceKey)
	}
}

// removeService removes a service from the reconciler
func (r *ClusterReconciler) removeService(obj interface{}) {
	service := obj.(*corev1.Service)
	serviceKey := fmt.Sprintf("%s/%s", service.Namespace, service.Name)

	r.servicePodsMu.Lock()
	defer r.servicePodsMu.Unlock()

	delete(r.servicePods, serviceKey)
	log.Printf("Removed service: %s", serviceKey)
}

// watchPods watches for changes to pods and updates service-to-pod mappings
func (r *ClusterReconciler) watchPods() {
	listWatch := cache.NewListWatchFromClient(
		r.clientset.CoreV1().RESTClient(),
		"pods",
		corev1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		listWatch,
		&corev1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    r.reconcilePod,
			UpdateFunc: func(oldObj, newObj interface{}) { r.reconcilePod(newObj) },
			DeleteFunc: r.removePod,
		},
	)

	log.Println("Starting Pod watcher...")
	controller.Run(make(chan struct{}))
}

// reconcilePod adds or updates a pod's endpoint for its service
func (r *ClusterReconciler) reconcilePod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return
	}

	r.servicePodsMu.Lock()
	defer r.servicePodsMu.Unlock()

	for serviceKey, servicePods := range r.servicePods {
		// Extract service namespace and name
		parts := strings.Split(serviceKey, "/")
		if len(parts) != 2 {
			continue
		}

		serviceNamespace := parts[0]
		serviceName := parts[1]

		// Fetch the service and match its selector to the pod labels
		service, err := r.clientset.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		selector := labels.Set(service.Spec.Selector).AsSelector()
		if selector.Matches(labels.Set(pod.Labels)) {
			// Add or update the pod in the service's endpoints
			endpoint := fmt.Sprintf("%s:%d", pod.Status.PodIP, 80) // Assuming HTTP on port 80
			if !contains(servicePods.PodEndpoints, endpoint) {
				servicePods.PodEndpoints = append(servicePods.PodEndpoints, endpoint)
				log.Printf("Reconciled pod: %s -> %s (Service: %s)", pod.Name, endpoint, serviceKey)
			}
		}
	}
}

// removePod removes a pod from its service's endpoints
func (r *ClusterReconciler) removePod(obj interface{}) {
	pod := obj.(*corev1.Pod)
	endpoint := fmt.Sprintf("%s:%d", pod.Status.PodIP, 80)

	r.servicePodsMu.Lock()
	defer r.servicePodsMu.Unlock()

	for serviceKey, servicePods := range r.servicePods {
		for i, ep := range servicePods.PodEndpoints {
			if ep == endpoint {
				// Remove the pod from the service's endpoints
				servicePods.PodEndpoints = append(servicePods.PodEndpoints[:i], servicePods.PodEndpoints[i+1:]...)
				log.Printf("Removed pod: %s (Service: %s)", endpoint, serviceKey)
				break
			}
		}
	}
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetNextPodEndpoint selects the next pod in a round-robin manner for a service
func (r *ClusterReconciler) GetNextPodEndpoint(serviceKey string) (string, error) {
	r.servicePodsMu.Lock()
	defer r.servicePodsMu.Unlock()

	servicePods, exists := r.servicePods[serviceKey]
	if !exists || len(servicePods.PodEndpoints) == 0 {
		return "", fmt.Errorf("no pods available for service %s", serviceKey)
	}

	endpoint := servicePods.PodEndpoints[servicePods.RoundRobinIdx]
	servicePods.RoundRobinIdx = (servicePods.RoundRobinIdx + 1) % len(servicePods.PodEndpoints)
	return endpoint, nil
}

// HTTP Server to Proxy Requests to Services in Round-Robin
func (r *ClusterReconciler) startHTTPServer(port int) {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		// Extract service key from the URL path
		parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		if len(parts) != 2 {
			http.Error(w, "Invalid URL path. Expected /<namespace>/<service-name>", http.StatusBadRequest)
			return
		}

		serviceKey := fmt.Sprintf("%s/%s", parts[0], parts[1])
		endpoint, err := r.GetNextPodEndpoint(serviceKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		// Proxy the request to the selected pod
		proxyURL := fmt.Sprintf("http://%s%s", endpoint, req.URL.Path)
		log.Printf("Forwarding request to pod: %s", proxyURL)

		resp, err := http.Get(proxyURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error forwarding to pod: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Copy the response back to the client
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	log.Printf("Starting HTTP server on port %d...", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func main() {
	reconciler, err := NewClusterReconciler()
	if err != nil {
		log.Fatalf("Failed to initialize reconciler: %v", err)
	}

	// Start the reconciler
	reconciler.Start()

	// Start the HTTP server
	reconciler.startHTTPServer(8080)
}
