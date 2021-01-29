# Auto Private Link
The Auto Private Link controller looks for annotated Kubernetes services and creates accompanying Azure Private Link resources using the ARM API. Also, the CRD included in this project allows the controller to automatically create endpoints for these services in specified subnets.  

![Architecture](images/architecture.png)


## Setup
Create an AKS cluster using the Azure CLI if you don't already have one 
```bash
#Resource Group
az group create --name private-link-test --location eastus

#Cluster 
az aks create --resource-group private-link-test --name apl-cluster --node-count 1 --generate-ssh-keys

#Connect to cluster
az aks get-credentials --resource-group private-link-test --name apl-cluster

```
Deploy an internally loadbalanced service to the AKS cluster. This will create an internal loadblancer in the AKS node resource group. Use the yaml below as an example. Please pay attention to the required annotations.

```bash
kubectl apply -f https://raw.githubusercontent.com/garvinmsft/auto-private-link/main/example/internal-service.yaml
```

```yaml
apiVersion: v1
kind: Service
metadata:
  #The Azure resource name will be the same as the service
  name: internal-app
  annotations:
    #Currently, only internal LB services are supported
    service.beta.kubernetes.io/azure-load-balancer-internal: "true"
    #The controller will only process services with this annotation
    garvinmsft.github.com/apl: "true"
spec:
  type: LoadBalancer
  ports:
  - port: 80
  selector:
    app: nginx
---


apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  selector:
    matchLabels:
       app: nginx
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80

```
### Private Link Requirements

The private link service requires a subnet to NAT traffic to the AKS cluster from private endpoints in outside VNETS. By default the `az aks create` command will create a vnet in the `10.0.0.0/8` range and will assign the cluster to a subnet in the `10.240.0.0/16` range. If the subnet does not exist and the Azure AD identity used by the controller has sufficient permissions it will create the subnet. This requires the `natSubnetPrefix` property to be set. Alternatively, the subnet can be created manually. This subnet can exist within the AKS VNET or any another VNET which is peered to the AKS VNET.

### Install Using Helm
Get required values related to the AKS cluster
```bash
aksClusterName="apl-cluster"
aksResourceGroup="private-link-test"

nodeResourceGroup=$(az aks show -n $aksClusterName -g $aksResourceGroup -o tsv --query "nodeResourceGroup")
aksVnetName=$(az network vnet list -g $nodeResourceGroup -o tsv --query "[0].name")

echo $nodeResourceGroup
echo $aksVnetName
```


Create a `vaules.yaml` file for the helm install

```yaml

image:
  repository: ghcr.io/garvinmsft/auto-private-link
  pullPolicy: IfNotPresent
  # Overrides the image tag whose default is the chart appVersion.
  tag: ""

imagePullSecrets: []
nameOverride: ""
fullnameOverride: ""
podAnnotations: {}

# All durations in seconds
kubernetes:
  syncPeriod: 30
  minRetrydelay: 5
  maxRetryDelay: 300

autoPrivateLink:
  serviceAnnotation: garvinmsft.github.com/apl
  network:
    #name of k8s vnet or vnet peered to k8s vnet used for NAT
    vnetName: <aksVnetName> # Change this

    #resource group of k8s vnet or vnet peered to k8s vnet use for NAT
    vnetResourceGroupName: <nodeResourceGroup> #Change this

     #name of subnet in  k8s vnet or vnet peered to k8s vnet used for private link NAT
    natSubnetName: apl-nat-subnet

     #address range for private link NAT. Only needed if subnet not already created
    natSubnetPrefix: 10.241.255.0/27

    #name of the internal kubernetes load balancer 
    loadBalancerName: kubernetes-internal 

    #resource group of the internal kubernetes load balancer
    loadBalancerResourceGroup: <nodeResourceGroup> #Change this 
armAuth:
  secretJSON: '<<Generate this value with: az ad sp create-for-rbac --sdk-auth | base64 -w0 >>'
```



```bash

helm repo add auto-private-link https://garvinmsft.github.io/auto-private-link
helm repo update
helm install --debug -f values.yaml auto-private-link auto-private-link/auto-private-link

```