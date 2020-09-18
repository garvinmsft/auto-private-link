package config

import (
	"errors"
)

var (
	//ErrorNoVnetResourceGroup is displayed when the resource group param is missing
	ErrorNoVnetResourceGroup = errors.New("Missing VNET Resource Group param")

	//ErrorNoVnetName is displayed when the vnet param is missing
	ErrorNoVnetName = errors.New("No vnet specified")

	//ErrorNoSubnetName is displayed when the subnet param is missing
	ErrorNoSubnetName = errors.New("No subnet specified")

	//ErrorNoSubnetPrefix is displayed when the resource group param is missing
	ErrorNoSubnetPrefix = errors.New("Sudnet prefix must be in cidr notation: y.y.y.y/z")

	//ErrorNoLoadBalancerResourceGroup is displayed when the load balancer resource group param is missing
	ErrorNoLoadBalancerResourceGroup = errors.New("Missing Resource Group param for load balancer")

	//ErrorNoLoadBalancer is displayed when the load balancer param is missing
	ErrorNoLoadBalancer = errors.New("Missing load balancer")

	//ErrorNoReconcilePeriod is displayed when the load balancer param is missing
	ErrorNoReconcilePeriod = errors.New("Missing reconcile period")

	//ErrorNoAzureConfigFile is displayed when the load balancer param is missing
	ErrorNoAzureConfigFile = errors.New("Missing azure config file location")

	//ErrorNoAzureRegion is displayed when the load balancer param is missing
	ErrorNoAzureRegion = errors.New("Missing azure region configuration")
)