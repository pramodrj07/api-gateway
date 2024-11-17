# API Gateway in Go

This is a sample implementation of an API Gateway using Go (Golang). The API Gateway acts as a reverse proxy, handling requests and routing them to appropriate microservices.

## Features
- Reverse proxy for microservices.
- Customizable request routing.
- Load balancing between multiple instances of a service using round-robin and least-connections algorithms.
- Integration with Docker for containerized deployments.

## Prerequisites
- Go (version 1.20 or later recommended)
- Docker (for containerized deployment)
- Make sure `GOPATH` is properly set up.

## Getting Started

### Clone the Repository
```bash
git clone https://github.com/pramodrj07/api-gateway.git
cd api-gateway
go mod tidy
go run main.go
```

### Build the Docker Image
```bash
cd cmd
docker build -t api-gateway .
# push the image to docker hub
docker tag api-gateway <user_name>/api-gateway
docker push <user_name>/api-gateway
```

### Use the Kubernetes Deployment to Test the Gateway
```bash
# Update the image in deployments/gateway.yaml with the image pushed to DockerHub
# The deployment files already have the image names that are tested and pushed to DockerHub
kubectl apply -f deployments/gateway.yaml

# Deploy the mock services
kubectl apply -f /workspaces/api-gateway/deployments/deployment-A-1st-instance.yaml
kubectl apply -f /workspaces/api-gateway/deployments/deployment-A-2nd-instance.yaml
kubectl apply -f /workspaces/api-gateway/deployments/deployment-A-3rd-instance.yaml
kubectl apply -f /workspaces/api-gateway/deployments/deployment-B.yaml
kubectl apply -f /workspaces/api-gateway/deployments/deployment-C.yaml

# Check the status of the pods and wait until they are running
kubectl get pods

# port forward the gateway servic
kubectl port-forward service/gateway-service 8080:8080

# In parallel, log the requests to the gateway
kubectl logs -f <name_of_the_gateway_pod>

# Test the gateway
curl -X GET http://localhost:8080/serviceA

# As serviceA has 3 instances, the gateway will route the request to one of the instances in a round-robin fashion.
# Repeat the above command multiple times to see the requests being routed to different instances.
```
