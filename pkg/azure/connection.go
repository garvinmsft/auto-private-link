package azure

import (
	"context"
	"fmt"
	apl "github.com/garvinmsft/auto-private-link/pkg/apis/apl/v1alpha1"
	n "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2020-05-01/network"
	"github.com/Azure/go-autorest/autorest/to"
)

const (
	approved  = "Approved"
	pending  = "Pending"
	privateEndpointSubnetError = "PrivateEndpointSubnetError"
	privateEndpointCreationError = "PrivateEndpointCreationError"
	privateEndpointCreated = "PrivateEndpointCreated"
)

//AddUpdatePrivateConnection adds or updates a private link endpoint
func (azCtx AzContext) AddUpdatePrivateConnection(conn *apl.ServiceConnection, serviceName string) error {

	ctx := context.TODO()
	
	subnet, err := azCtx.getPrivateEndpointSubnet(conn)

	if err != nil {
		azCtx.warningEvent(conn, privateEndpointSubnetError, err.Error())
		return err
	}
	
	ep, err := azCtx.getOrCreateEndpoint(conn, serviceName, subnet)

	if err!=nil {
		return err
	}

	if len(*ep.PrivateEndpointProperties.ManualPrivateLinkServiceConnections) == 0 {
		return fmt.Errorf("No connections found on endpoint. This should never happen?")
	}

	connStatus := (*ep.PrivateEndpointProperties.ManualPrivateLinkServiceConnections)[0].PrivateLinkServiceConnectionState.Status
	
	//No need to proceed if the status is approved.
	if *connStatus == approved {
		return nil
	} 

	//TODO: Other statuses may require delete and recreate. Deal with that later
	if *connStatus != pending {
		return fmt.Errorf("The status of this connection is %v", *connStatus)
	} 

	//Proceed with manual approval
	cons, err := azCtx.PrivateLinkServicesClient.ListPrivateEndpointConnections(ctx,
		azCtx.cfg.LoadBalancerResourceGroup,
		serviceName)
	
	if err!= nil {
		return err
	}

	var connName string 
	for _, v := range cons.Values() {
		print(*v.PrivateEndpointConnectionProperties.PrivateEndpoint.ID)
		if *v.PrivateEndpoint.ID == *ep.ID {
			connName = *v.Name
			break
		}
	}

	if connName == "" {
		return fmt.Errorf("Could not find connection in: %v for endpoint: %v", serviceName, ep.Name)
	}

	_, err = azCtx.PrivateLinkServicesClient.UpdatePrivateEndpointConnection(ctx,
		azCtx.cfg.LoadBalancerResourceGroup,
		serviceName,
		connName,
		n.PrivateEndpointConnection{
			Name: &connName,
			PrivateEndpointConnectionProperties: &n.PrivateEndpointConnectionProperties{
				PrivateLinkServiceConnectionState: &n.PrivateLinkServiceConnectionState{
					Status: to.StringPtr(approved),
				},
			},
		},
	)

	if err!=nil {
		return err
	}
	
	return nil
}

func (azCtx AzContext) getPrivateEndpointSubnet(conn *apl.ServiceConnection) (n.Subnet, error) {
	
	ctx := context.TODO()

	subnet, err := azCtx.SubnetClient.Get(ctx, 
					conn.Spec.ResourceGroup, 
					conn.Spec.VnetName,
					conn.Spec.SubnetName, "")
					
	if err != nil {
		return subnet, err
	}

	//fix policy setting
	if *subnet.PrivateEndpointNetworkPolicies != policyDisabled { 
		future, _ := azCtx.SubnetClient.CreateOrUpdate(ctx,
			conn.Spec.ResourceGroup,
			conn.Spec.VnetName,
			conn.Spec.SubnetName,
			n.Subnet{
				SubnetPropertiesFormat: &n.SubnetPropertiesFormat{
					PrivateEndpointNetworkPolicies: &policyDisabled,
					AddressPrefix: subnet.AddressPrefix,
				},
			},
			
		)

		err = future.WaitForCompletionRef(ctx, azCtx.PrivateEndpointsClient.Client)

		if err != nil {
			return subnet, err
		} 

		subnet, err = future.Result(azCtx.SubnetClient)

		if err != nil {
			return subnet, err
		}
	}

	return subnet, nil
}

func (azCtx AzContext) getOrCreateEndpoint(conn *apl.ServiceConnection, serviceName string, subnet n.Subnet) (n.PrivateEndpoint, error) {
	ctx := context.TODO()

	ep, err := azCtx.PrivateEndpointsClient.Get(ctx, conn.Spec.ResourceGroup, conn.Name, "")

	if err != nil && ep.Response.Response.StatusCode != 404 {
		return ep, err
	} 
	
	if err == nil {
		return ep, nil
	}

	ep, err = azCtx.createEndpoint(conn, serviceName, subnet)

	if err!= nil {
		azCtx.warningEvent(conn, privateEndpointCreationError, err.Error())
		return ep, err
	}

	azCtx.successEvent(conn, privateEndpointCreated, *ep.ID)
	return ep, nil

}

func (azCtx AzContext) createEndpoint(conn *apl.ServiceConnection, serviceName string, subnet n.Subnet) (n.PrivateEndpoint, error) {

	ctx := context.TODO()
	var ep n.PrivateEndpoint

	//get service ID (can this exist if the endoint doesn't?)
	pls, err := azCtx.PrivateLinkServicesClient.Get(ctx, azCtx.cfg.LoadBalancerResourceGroup, serviceName, "")

	if err!= nil {
		return ep, err
	}

	future, err := azCtx.PrivateEndpointsClient.CreateOrUpdate(ctx,
		conn.Spec.ResourceGroup,
		conn.Name,
		n.PrivateEndpoint{
			Name: &conn.Name,
			Location: &azCtx.Location,
			PrivateEndpointProperties: &n.PrivateEndpointProperties{
				Subnet: &n.Subnet{
					ID: subnet.ID,
				},
				ManualPrivateLinkServiceConnections: &[]n.PrivateLinkServiceConnection{
					{
						Name: &conn.Name,
						PrivateLinkServiceConnectionProperties: &n.PrivateLinkServiceConnectionProperties{
							PrivateLinkServiceID: pls.ID,
							
						},
					},
				},
			},
		},
	)

	if err != nil {
		return ep, err
	}

	err = future.WaitForCompletionRef(ctx, azCtx.PrivateEndpointsClient.Client)

	if err != nil {
		return ep, err
	}

	ep, err = future.Result(azCtx.PrivateEndpointsClient)

	if err != nil {
		return ep, err
	}

	return ep, nil
}

//RemoveEndpoint Deletes a private endpoint
func (azCtx AzContext) RemoveEndpoint(conn *apl.ServiceConnection) error {

	ctx := context.TODO()

	future, err := azCtx.PrivateEndpointsClient.Delete(ctx,
		conn.Spec.ResourceGroup,
		conn.Name,
		)

	//404 on delete shouldn't return an error correct?
	if err != nil {
		return err
	}
	
	err = future.WaitForCompletionRef(ctx, azCtx.PrivateEndpointsClient.Client)
	
	if err != nil {
			return err
	}

	return nil

}