package proxmox

import (
	"fmt"
	"regexp"
	"strconv"
	"sync"

	pxapi "github.com/enix/proxmox-api-go/proxmox"
	"github.com/hashicorp/terraform/helper/schema"
)

type providerConfiguration struct {
	Client          *pxapi.Client
	MaxParallel     int
	CurrentParallel int
	MaxVMID         int
	Mutex           *sync.Mutex
	Cond            *sync.Cond
}

// Provider - Terrafrom properties for proxmox
func Provider() *schema.Provider {
	return &schema.Provider{

		Schema: map[string]*schema.Schema{
			"api_username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PROXMOX_USER", nil),
				Description: "username, maywith with @pam",
			},
			"api_password": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PROMOX_PASS", nil),
				Description: "secret",
				Sensitive:   true,
			},
			"api_url": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("PM_API_URL", nil),
				Description: "https://host.fqdn:8006/api2/json",
			},
			"api_tls_insecure": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"api_parallel_clones": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"api_parallel_resizes": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},

			"parallel_resources": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  4,
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"proxmox_vm_qemu": resourceVmQemu(),
			// TODO - storage_iso
			// TODO - bridge
			// TODO - vm_qemu_template
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (configuration interface{}, err error) {
	client, err := pxapi.NewClient(&pxapi.Configuration{
		Url:			d.Get("api_url").(string),	
		Username:		d.Get("api_username").(string),
		Password:		d.Get("api_password").(string),
		TlsInsecure:	d.Get("api_tls_insecure").(bool),
		ParallelClone:	d.Get("api_parallel_clones").(bool),
		ParallelResize:	d.Get("api_parallel_resizes").(bool),
		}, true)

	if err != nil {
		return nil, err
	}
	var mut sync.Mutex
	return &providerConfiguration{
		Client:          client,
		MaxParallel:     d.Get("parallel_resources").(int),
		CurrentParallel: 0,
		MaxVMID:         -1,
		Mutex:           &mut,
		Cond:            sync.NewCond(&mut),
	}, nil
}

func nextVmId(pconf *providerConfiguration) (nextId int, err error) {
	pconf.Mutex.Lock()
	pconf.MaxVMID, err = pconf.Client.GetNextID(pconf.MaxVMID + 1)
	if err != nil {
		return 0, err
	}
	nextId = pconf.MaxVMID
	pconf.Mutex.Unlock()
	return nextId, nil
}

func pmParallelBegin(pconf *providerConfiguration) {
	pconf.Mutex.Lock()
	for pconf.CurrentParallel >= pconf.MaxParallel {
		pconf.Cond.Wait()
	}
	pconf.CurrentParallel++
	pconf.Mutex.Unlock()
}

func pmParallelEnd(pconf *providerConfiguration) {
	pconf.Mutex.Lock()
	pconf.CurrentParallel--
	pconf.Cond.Signal()
	pconf.Mutex.Unlock()
}

func resourceId(targetNode string, resType string, vmId int) string {
	return fmt.Sprintf("%s/%s/%d", targetNode, resType, vmId)
}

var rxRsId = regexp.MustCompile("([^/]+)/([^/]+)/(\\d+)")

func parseResourceId(resId string) (targetNode string, resType string, vmId int, err error) {
	idMatch := rxRsId.FindStringSubmatch(resId)
	if idMatch == nil {
		err = fmt.Errorf("Invalid resource id: %s", resId)
	}
	targetNode = idMatch[1]
	resType = idMatch[2]
	vmId, err = strconv.Atoi(idMatch[3])
	return
}
