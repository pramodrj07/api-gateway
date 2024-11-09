package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	controller "github.com/pramodrj07/api-gateway/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	// Parse flags
	var metricsAddr, apiGatewayAddr string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&apiGatewayAddr, "api-gateway-addr", ":8081", "The address the API gateway binds to.")
	var enableLeaderElection bool
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false, "Enable leader election for controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create a new Manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Metrics:          server.Options{BindAddress: metricsAddr},
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "service-discovery-controller",
	})
	if err != nil {
		os.Exit(1)
	}

	// Shared service map and mutex for API gateway and controller
	serviceMap := make(map[string]string)
	var mutex sync.RWMutex

	// Set up the Service controller
	reconciler := &controller.ServiceReconciler{
		Client:     mgr.GetClient(),
		ServiceMap: serviceMap,
		Mutex:      mutex,
	}
	if err = reconciler.SetupWithManager(mgr); err != nil {
		os.Exit(1)
	}

	// Initialize API Gateway
	apiGateway := controller.NewAPIGateway(serviceMap, &mutex)

	// Start the API gateway in a separate goroutine
	go func() {
		fmt.Printf("Starting API Gateway at %s\n", apiGatewayAddr)
		if err := http.ListenAndServe(apiGatewayAddr, apiGateway); err != nil {
			log.Fatalf("API Gateway failed to start: %v", err)
		}
	}()

	// Start the Manager
	fmt.Println("Starting the Service Discovery Manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		os.Exit(1)
	}
}
