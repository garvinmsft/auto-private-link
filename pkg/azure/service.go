package azure

import (
	"context"
	n "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	v1 "k8s.io/api/core/v1"
	"fmt"
)


const (
	privateLinkServiceCreationError = "PrivateLinkServiceCreationError"
	privateLinkServiceCreated = "PrivateLinkServiceCreated"
	privateLinkServiceError = "PrivateLinkServiceError"
	privateLinkServiceRemoved = "PrivateLinkServiceRemoved"
	msgPrivateLinkServiceRemoved = "Private link service deleted!"
	privateLinkServiceRemovalError = "PrivateLinkServiceRemovalError"
	natSubnetCreated = "NatSubnetCreated"
	natSubnetCreationError = "NatSubnetCreationError"

)

//AddUpdatePrivateService adds or updates a private link service
func (azCtx AzContext) AddUpdatePrivateService(service *v1.Service) error {

	exists, err := azCtx.privateLinkServiceExists(service)

	if err!=nil {
		return err
	}

	//Is there anything that may require an update in the resource? Don't think so
	if exists {
		return nil
	}

	subnet , err := azCtx.getOrCreateNatSubnet(service)

	if err != nil {
		return err
	}

	frontEndID, err := azCtx.getLoadBalancerFrontendIDForIP(service)

	if err!=nil {
		return err
	}

	aplID, err := azCtx.createPrivateLinkService(service, frontEndID, *subnet.ID)
	
	if err!= nil {
		azCtx.warningEvent(service, privateLinkServiceCreationError, err.Error())
	}
	
	azCtx.successEvent(service, privateLinkServiceCreated, aplID)

	return nil
}

func (azCtx AzContext) getLoadBalancerFrontendIDForIP(service *v1.Service) (string, error){
	ctx := context.TODO() 
		
	lfcList, err := azCtx.LbFrontEndConfigClient.List(ctx, 
		azCtx.cfg.LoadBalancerResourceGroup, 
		azCtx.cfg.LoadBalancerName)
	
	if err!=nil {
		return "", err
	}

	var frontEndID string

	for _,i := range lfcList.Values() {
		if service.Status.LoadBalancer.Ingress[0].IP == *i.PrivateIPAddress{
			frontEndID = *i.ID
		}
	}

	if frontEndID == "" {
		return "", fmt.Errorf("Could not find service ip in the load balancer")
	}

	return frontEndID, nil
}

func (azCtx AzContext) privateLinkServiceExists(service *v1.Service) (bool, error) {
	ctx:= context.TODO()

	result, err := azCtx.PrivateLinkServicesClient.Get(ctx, azCtx.cfg.LoadBalancerResourceGroup, service.Name, "")

	//3 possible states. There could be a permission error for example.
	if err != nil {
		if result.Response.Response.StatusCode == 404 {
			return false, nil
		}
		return false, err
	} 

	return true, nil

}

func (azCtx AzContext) createPrivateLinkService(service *v1.Service, frontEndID string, subnetID string ) (string, error) {

	ctx:= context.TODO()

	future, err := azCtx.PrivateLinkServicesClient.CreateOrUpdate(ctx, azCtx.cfg.LoadBalancerResourceGroup, service.Name, 
		n.PrivateLinkService{
			Name: &service.Name,
			Location: &azCtx.Location,
			PrivateLinkServiceProperties: &n.PrivateLinkServiceProperties{
					LoadBalancerFrontendIPConfigurations: &[]n.FrontendIPConfiguration{
					{
						ID: &frontEndID,
					},
				},
				IPConfigurations: &[]n.PrivateLinkServiceIPConfiguration{
					{
						PrivateLinkServiceIPConfigurationProperties: &n.PrivateLinkServiceIPConfigurationProperties{
							Subnet: &n.Subnet{
								ID: &subnetID,
							},
						},
						Name: &service.Name,//should be unique accross namespaces unless namespace appended
					},
					
				},
			},
		},
	)

	if err != nil {
		return "", err
	}

	err = future.WaitForCompletionRef(ctx, azCtx.PrivateLinkServicesClient.Client)

	if err!= nil {
		return "", err
	}
	aplService, err := future.Result(azCtx.PrivateLinkServicesClient)

	if err != nil {
		return "", err
	}

	return *aplService.ID, nil
}

func (azCtx AzContext) createNatSubnet(service *v1.Service) (n.Subnet, error) {

	ctx := context.TODO()

	var subnet n.Subnet

	future, err := azCtx.SubnetClient.CreateOrUpdate(ctx,
		azCtx.cfg.VnetResourceGroupName,
		azCtx.cfg.VnetName,
		azCtx.cfg.NatSubnetName,
		n.Subnet{
			SubnetPropertiesFormat: &n.SubnetPropertiesFormat{
				AddressPrefix: &azCtx.cfg.NatSubnetPrefix,
				PrivateLinkServiceNetworkPolicies: &policyDisabled,
			},
		},
	)

	//Generics?
	if err != nil {
		return subnet, err
	}

	err = future.WaitForCompletionRef(ctx, azCtx.SubnetClient.Client)

	if err != nil{
		return subnet, err
	}

	subnet, err = future.Result(azCtx.SubnetClient)

	if err != nil {
		return subnet, err
	}

	return subnet, nil
	
}

//GetNatSubnetID gets the id of the NAT subnet. Create it if it doesn't exist
func (azCtx AzContext) getOrCreateNatSubnet(service *v1.Service) (n.Subnet, error) {

	ctx := context.TODO()

	//Get the NAT subnet if it exists
	subnet, err := azCtx.SubnetClient.Get(ctx, 
		azCtx.cfg.VnetResourceGroupName, 
		azCtx.cfg.VnetName, 
		azCtx.cfg.NatSubnetName,"")

	if err != nil && subnet.Response.Response.StatusCode != 404 {
		return subnet, err
	} 

	if err == nil {
		return subnet, nil
	}

	subnet, err = azCtx.createNatSubnet(service)

	if err!=nil {
		azCtx.warningEvent(service, natSubnetCreationError, err.Error())
	}
	
	azCtx.successEvent(service, natSubnetCreated, *subnet.ID)

	return subnet, nil
}


//RemoveService removes a private link service if it exists
func (azCtx AzContext) RemoveService(service *v1.Service) error {

	ctx := context.TODO()

	apl, err := azCtx.PrivateLinkServicesClient.Get(ctx, 
		azCtx.cfg.LoadBalancerResourceGroup,
		service.Name,
		"",
	)
	

	if err != nil {
		if apl.Response.Response.StatusCode == 404  {
			return nil
		}

		return err
	}

	for _, item := range *apl.PrivateEndpointConnections {
		future, err := azCtx.PrivateEndpointsClient.Delete(ctx,
			azCtx.cfg.LoadBalancerResourceGroup,
			*item.PrivateEndpoint.Name,
		)

		if err != nil {
			return err
		}

		err = future.WaitForCompletionRef(ctx, azCtx.PrivateEndpointsClient.Client)

		if err != nil {
			return err
		}
	}

	future, err := azCtx.PrivateLinkServicesClient.Delete(ctx,
		azCtx.cfg.LoadBalancerResourceGroup,
		service.Name,
	)

	if err != nil {
		return err
	}

	err = future.WaitForCompletionRef(ctx, azCtx.PrivateLinkServicesClient.Client)

	if err != nil {
		azCtx.warningEvent(service, privateLinkServiceRemovalError, err.Error())
		return err
	}

	azCtx.successEvent(service, privateLinkServiceRemoved, msgPrivateLinkServiceRemoved)
	return nil
}


