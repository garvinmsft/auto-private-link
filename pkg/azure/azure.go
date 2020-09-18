package azure

import (
	"context"
	n "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/garvinmsft/auto-private-link/pkg/config"
	"k8s.io/client-go/tools/record"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "k8s.io/api/core/v1"
)

var (
	policyDisabled = "Disabled" 
)

//AzContext is the holder of all az api clients
type AzContext struct {
	SubnetClient n.SubnetsClient
	PrivateLinkServicesClient n.PrivateLinkServicesClient
	PrivateEndpointsClient  n.PrivateEndpointsClient
	LbFrontEndConfigClient n.LoadBalancerFrontendIPConfigurationsClient
	recorder record.EventRecorder
	Location string
	cfg config.Config
}


//NewAzContext creates a new azure api client
func NewAzContext(cfg config.Config, recorder record.EventRecorder) (AzContext, error) {

	azCtx := AzContext{
		cfg: cfg,
		recorder: recorder,
	}

	settings, err := auth.GetSettingsFromFile()
	if err!= nil{
		return azCtx, err
	}

	authorizer, err := auth.NewAuthorizerFromFile(n.DefaultBaseURI)
	if err!= nil{
		return azCtx, err
	}
	
	vnetClient := n.NewVirtualNetworksClient(settings.GetSubscriptionID())
	vnetClient.Authorizer = authorizer

	vnet, err := vnetClient.Get(context.TODO(), 
				cfg.VnetResourceGroupName,
				cfg.VnetName,"")

	if err!= nil {
		return azCtx, err
	}

	azCtx.Location = *vnet.Location
	azCtx.SubnetClient = n.NewSubnetsClient(settings.GetSubscriptionID())
	azCtx.PrivateLinkServicesClient = n.NewPrivateLinkServicesClient(settings.GetSubscriptionID())
	azCtx.PrivateEndpointsClient = n.NewPrivateEndpointsClient(settings.GetSubscriptionID()) 
	azCtx.LbFrontEndConfigClient =  n.NewLoadBalancerFrontendIPConfigurationsClient(settings.GetSubscriptionID())
	
	azCtx.SubnetClient.Authorizer = authorizer
	azCtx.PrivateLinkServicesClient.Authorizer = authorizer
	azCtx.PrivateEndpointsClient.Authorizer = authorizer
	azCtx.LbFrontEndConfigClient.Authorizer = authorizer

	return azCtx, nil
}

func(azCtx AzContext) successEvent(object runtime.Object, reason string, message string){
	azCtx.recorder.Event(object, v1.EventTypeNormal, reason, message )
}

func(azCtx AzContext) warningEvent(object runtime.Object, reason string, message string){
	azCtx.recorder.Event(object, v1.EventTypeWarning, reason, message)
}