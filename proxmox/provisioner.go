package proxmox

import (
	"context"
	"fmt"
	"time"

	pxapi "github.com/enix/proxmox-api-go/proxmox"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

// Provisioner - Terrafrom properties for proxmox-provisioner
func Provisioner() terraform.ResourceProvisioner {
	return &schema.Provisioner{
		Schema: map[string]*schema.Schema{
			"action": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"net1": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
		},

		ApplyFunc: applyFn,
	}
}

var currentClient *pxapi.Client

func applyFn(ctx context.Context) error {
	data := ctx.Value(schema.ProvConfigDataKey).(*schema.ResourceData)
	state := ctx.Value(schema.ProvRawStateKey).(*terraform.InstanceState)

	connInfo := state.Ephemeral.ConnInfo

	act := data.Get("action").(string)
	targetNode, _, vmID, err := parseResourceId(state.ID)
	if err != nil {
		return err
	}
	vmr := pxapi.NewVmRef(vmID)
	vmr.SetNode(targetNode)
	client := currentClient
	if client == nil {
		client, err := pxapi.NewClient(&pxapi.Configuration{
			Url:			connInfo["api_url"],	
			Username:		connInfo["api_username"],
			Password:		connInfo["api_password"],
			TlsInsecure:	connInfo["api_tls_insecure"] == "true",
			}, true)
		if err != nil {
			return err
		}
		currentClient = client
	}
	switch act {
	case "sshbackward":
		return pxapi.RemoveSshForwardUsernet(vmr, client)

	case "reconnect":
		err = pxapi.RemoveSshForwardUsernet(vmr, client)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Second)
		vmParams := map[string]interface{}{
			"net1": data.Get("net1").(string),
		}
		_, err = client.SetVmConfig(vmr, vmParams)
		time.Sleep(10 * time.Second)
		return err
	default:
		return fmt.Errorf("Unkown action: %s", act)
	}
}
