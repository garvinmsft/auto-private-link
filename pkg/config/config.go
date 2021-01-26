package config

import (
	"os"
	"net"
	"strconv"
	"time"
	v1 "k8s.io/api/core/v1"
)

const (
	//DefaultServiceAnnotation is the annotation used to include a service in auto private link
	DefaultServiceAnnotation  = "garvinmsft.github.com/apl"
	
	//DefaultSyncPeriod is the default sync period (in seconds) for watching resources
	DefaultSyncPeriod = 30

	//SyncPeriodEnvName the amount of time in between syncs
	SyncPeriodEnvName = "SYNC_DELAY_SECONDS"

	//DefaultMinRetryDelay is the default minimum wait time (in seconds) to retry sync
	DefaultMinRetryDelay = 5

	//MinRetryDelayEnvName the minimum amount of time (in seconds) to retry sync
	MinRetryDelayEnvName = "MIN_RETRY_DELAY_SECONDS"

	//DefaultMaxRetryDelay is the default maximum wait time (in seconds) to wait before retry sync
	DefaultMaxRetryDelay = 300

	//MaxRetryDelayEnvName the maximum amount of time (in seconds) to wait before retry sync
	MaxRetryDelayEnvName = "MAX_RETRY_DELAY_SECONDS"

	//ServiceAnnotationEnvName name of the annotation the controller will used to select APL services
	ServiceAnnotationEnvName = "SERVICE_ANNOTATION"

	//VnetResourceGroupEnvName resource group where the vnet is located
	VnetResourceGroupEnvName = "KUB_VNET_RESOURCE_GROUP_NAME"

	//VnetEnvName is the name of the vnet conaining the apl NAT subnet
	VnetEnvName = "KUB_VNET_NAME"

	//NatSubnetEnvName is the key for the name of he subnet used for private link service. 
	NatSubnetEnvName = "NAT_SUBNET_NAME"

	//NatSubnetPrefixEnvName is the cidr value used for the apl NAT subnet (Required if submit doesn't exist)
	NatSubnetPrefixEnvName = "NAT_SUBNET_PREFIX"

	//LoadBalancerResourceGroupEnvName the name of the resource group containing the kubernetes internal load balancer
	LoadBalancerResourceGroupEnvName = "KUB_INTERNAL_LOADBALANCER_RESOURCE_GROUP"

	//LoadBalancerEnvName the name of the kubernetes internal load balancer
	LoadBalancerEnvName = "KUB_INTERNAL_LOADBALANCER_NAME"

	//AzureAuthLocationEnvName Location of the azure auth config file
	AzureAuthLocationEnvName = "AZURE_AUTH_LOCATION"

	//AplPodEnvName name of pod currently running this controller
	AplPodEnvName = "APL_POD_NAME"

	//AplPodNamespaceEnvName name of pod currently running this controller
	AplPodNamespaceEnvName = "APL_POD_NAMESPACE_NAME"

)

//Config contains all variables needed to run a unique instance of the controller 
type Config struct {
	VnetResourceGroupName string
	VnetName string
	NatSubnetName string
	NatSubnetPrefix string
	LoadBalancerResourceGroup string
	LoadBalancerName string
	SyncPeriod time.Duration
	MinRetryDelay time.Duration
	MaxRetryDelay time.Duration
	ServiceAnnotation string
	AzureAuthLocation string
	APlPod *v1.Pod
}

//NewConfigFromEnv get config from env
func NewConfigFromEnv() (Config, error) {
	cfg := Config{
		VnetResourceGroupName: os.Getenv(VnetResourceGroupEnvName),
		VnetName: os.Getenv(VnetEnvName),
		NatSubnetName: os.Getenv(NatSubnetEnvName),
		NatSubnetPrefix: os.Getenv(NatSubnetPrefixEnvName),
		LoadBalancerResourceGroup: os.Getenv(LoadBalancerResourceGroupEnvName),
		LoadBalancerName: os.Getenv(LoadBalancerEnvName),
		ServiceAnnotation: os.Getenv(ServiceAnnotationEnvName),
		AzureAuthLocation: os.Getenv(AzureAuthLocationEnvName),
	}

	if i, err := strconv.Atoi(os.Getenv(SyncPeriodEnvName)); err == nil{
		cfg.SyncPeriod = time.Duration(i) * time.Second
	} else {
		cfg.SyncPeriod = time.Duration(DefaultSyncPeriod) * time.Second
	}

	if i, err := strconv.Atoi(os.Getenv(MinRetryDelayEnvName)); err == nil{
		cfg.MinRetryDelay = time.Duration(i) * time.Second  
	} else {
		cfg.MinRetryDelay = time.Duration(DefaultMinRetryDelay) * time.Second
	}

	if i, err := strconv.Atoi(os.Getenv(MaxRetryDelayEnvName)); err == nil{
		cfg.MaxRetryDelay = time.Duration(i)
	} else {
		cfg.MaxRetryDelay = time.Duration(DefaultMaxRetryDelay) * time.Second
	}

	if cfg.ServiceAnnotation == "" {
		cfg.ServiceAnnotation = DefaultServiceAnnotation
	}

	if err := cfg.parse(); err != nil {
		return cfg, err
	} 

	return cfg, nil
}

func (cfg* Config) parse() error {
	if cfg.VnetResourceGroupName == "" {
		return ErrorNoVnetResourceGroup
	}

	if cfg.VnetName == "" {
		return ErrorNoVnetName
	}

	if cfg.NatSubnetName == "" {
		return ErrorNoSubnetName
	}

	if _,_,err := net.ParseCIDR(cfg.NatSubnetPrefix); cfg.NatSubnetPrefix != "" && err != nil{
		return ErrorNoSubnetPrefix
	} 

	if cfg.LoadBalancerResourceGroup == ""{
		return ErrorNoLoadBalancerResourceGroup
	}

	if cfg.LoadBalancerName == "" {
		return ErrorNoLoadBalancer
	}

	if cfg.AzureAuthLocation == "" {
		return ErrorNoAzureConfigFile
	}


	return nil
}