package proxmox

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	pxapi "github.com/enix/proxmox-api-go/proxmox"
	"github.com/hashicorp/terraform/helper/schema"
)

const vmType = "qemu"

func resourceVmQemu() *schema.Resource {
	*pxapi.Debug = true
	return &schema.Resource{
		Create: resourceVmQemuCreate,
		Read:   resourceVmQemuRead,
		Update: resourceVmQemuUpdate,
		Delete: resourceVmQemuDelete,
		Exists: resourceVmQemuExists,
		// Importer: &schema.ResourceImporter{
		// 	State: resourceVmQemuImport,
		// },

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true, // as we want the VM hostname to be consistent
			},
			"desc": {
				Type:     schema.TypeString,
				Optional: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.TrimSpace(old) == strings.TrimSpace(new)
				},
			},
			"target_node": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"onboot": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"iso": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"clone": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"qemu_os": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "l26",
			},
			"memory": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"cores": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"sockets": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"network": &schema.Schema{
				Type:          schema.TypeList,
				Optional:      true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"model": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"macaddr": &schema.Schema{
							// TODO: Find a way to set MAC address in .tf config.
							Type:     schema.TypeString,
							Computed: true,
						},
						"bridge": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Default:  "nat",
						},
						"tag": &schema.Schema{
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "VLAN tag.",
							Default:     -1,
						},
						"firewall": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"rate": &schema.Schema{
							Type:     schema.TypeInt,
							Optional: true,
							Default:  -1,
						},
						"queues": &schema.Schema{
							Type:     schema.TypeInt,
							Optional: true,
							Default:  -1,
						},
						"link_down": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
			},
			"disk": &schema.Schema{
				Type:          schema.TypeList,
				Optional:      true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"storage": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"storage_type": &schema.Schema{
							Type:        schema.TypeString,
							Optional:    true,
							Default:     "dir",
							Description: "One of PVE types as described: https://pve.proxmox.com/wiki/Storage",
						},
						"size": &schema.Schema{
							Type:     schema.TypeString,
							Required: true,
						},
						"format": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Default:  "raw",
						},
						"cache": &schema.Schema{
							Type:     schema.TypeString,
							Optional: true,
							Default:  "none",
						},
						"backup": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"iothread": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"replicate": &schema.Schema{
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
					},
				},
			},
			"ssh_forward_ip": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"ssh_user": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"ssh_private_key": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.TrimSpace(old) == strings.TrimSpace(new)
				},
			},
			"cloudinit_user": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"cloudinit_password": {
				Type:     schema.TypeString,
				Optional: true,
				StateFunc: func(val interface{}) string {
					return pxapi.HiddenPassword
				},
			},
			"cloudinit_searchdomain": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"cloudinit_nameserver": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"cloudinit_sshkeys": {
				Type:     schema.TypeString,
				Optional: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.TrimSpace(old) == strings.TrimSpace(new)
				},
			},
			"cloudinit_ipconfig0": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"cloudinit_ipconfig1": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

var rxIPconfig = regexp.MustCompile("ip6?=([0-9a-fA-F:\\.]+)")

func resourceVmQemuCreate(d *schema.ResourceData, meta interface{}) (err error) {
	pconf := meta.(*providerConfiguration)
	pmParallelBegin(pconf)
	defer pmParallelEnd(pconf)
	client := pconf.Client

	// generate Proxmox configuration from Terraform configuration
	var config *pxapi.ConfigQemu
	config, err = state2ConfigQemu(d)
	if err != nil {
		return
	}

	log.Print("[DEBUG] get next VmId")
	var nextId int
	nextId, err = nextVmId(pconf)
	if err != nil {
		return
	}

	vmr := pxapi.NewVmRef(nextId)
	vmr.SetNode(d.Get("target_node").(string))
	vmr.SetVmType(vmType)

	d.Partial(true)

	// check if ISO or clone
	if d.Get("clone").(string) != "" {
		var sourceVmr *pxapi.VmRef
		sourceVmr, err = client.GetVmRefByName(d.Get("clone").(string))
		if err != nil {
			return
		}
		log.Print("[DEBUG] cloning VM")
		err = config.CloneVm(sourceVmr, vmr, client)
		if err != nil {
			return
		}
		d.SetId(strconv.Itoa(vmr.VmId()))
		d.SetPartial("target_node")
		d.SetPartial("name")
		d.SetPartial("clone")

		err = config.UpdateConfig(vmr, client)
		if err != nil {
			return
		}
		d.SetPartial("network")
		d.SetPartial("memory")
		d.SetPartial("cloudinit_user")
		d.SetPartial("cloudinit_password")
		d.SetPartial("cloudinit_searchdomain")
		d.SetPartial("cloudinit_nameserver")
		d.SetPartial("cloudinit_sshkeys")
		d.SetPartial("cloudinit_ipconfig0")
		d.SetPartial("cloudinit_ipconfig1")

		// give sometime to proxmox to catchup
		time.Sleep(5 * time.Second)

		err = prepareDiskSize(client, vmr, devicesList2QemuDevices(d.Get("disk").([]interface{})))
		if err != nil {
			return
		}
		d.SetPartial("disk")
	} else if d.Get("iso").(string) != "" {
		config.QemuIso = d.Get("iso").(string)
		err = config.CreateVm(vmr, client)
		if err != nil {
			return
		}
		d.SetId(strconv.Itoa(vmr.VmId()))
		d.SetPartial("target_node")
		d.SetPartial("name")
	}
	d.Partial(false)

	// give sometime to proxmox to catchup
	time.Sleep(5 * time.Second)

	log.Print("[DEBUG] starting VM")
	_, err = client.StartVm(vmr)
	if err != nil {
		return
	}
	return
}

func resourceVmQemuUpdate(d *schema.ResourceData, meta interface{}) (err error) {
	pconf := meta.(*providerConfiguration)
	pmParallelBegin(pconf)
	defer pmParallelEnd(pconf)
	client := pconf.Client

	// generate Proxmox configuration from Terraform configuration
	var config *pxapi.ConfigQemu
	config, err = state2ConfigQemu(d)
	if err != nil {
		return
	}

	vmId, _ := strconv.Atoi(d.Id())
	vmr := pxapi.NewVmRef(vmId)
	vmr.SetNode(d.Get("target_node").(string))
	vmr.SetVmType(vmType)

	err = config.UpdateConfig(vmr, client)
	if err != nil {
		return
	}

	// give sometime to proxmox to catchup
	time.Sleep(5 * time.Second)

	prepareDiskSize(client, vmr, devicesList2QemuDevices(d.Get("disk").([]interface{})))

	// give sometime to proxmox to catchup
	time.Sleep(5 * time.Second)

	// Start VM only if it wasn't running.
	vmState, err := client.GetVmState(vmr)
	if err != nil {
		return
	}
	if vmState["status"] == "stopped" {
		log.Print("[DEBUG] starting VM")
		_, err = client.StartVm(vmr)
	}
	// give sometime to bootup
	time.Sleep(9 * time.Second)
	return
}

func resourceVmQemuRead(d *schema.ResourceData, meta interface{}) (err error) {
	pconf := meta.(*providerConfiguration)
	pmParallelBegin(pconf)
	defer pmParallelEnd(pconf)
	client := pconf.Client

	vmId, _ := strconv.Atoi(d.Id())
	vmr := pxapi.NewVmRef(vmId)
	vmr.SetNode(d.Get("target_node").(string))
	vmr.SetVmType(vmType)

	config, err := pxapi.NewConfigQemuFromApi(vmr, client)
	if err != nil {
		return
	}

	d.SetId(strconv.Itoa(vmId))
	d.Set("target_node", vmr.Node())
	d.Set("name", config.Name)
	d.Set("desc", config.Description)
	d.Set("onboot", config.Onboot)
	d.Set("memory", config.Memory)
	d.Set("cores", config.QemuCores)
	d.Set("sockets", config.QemuSockets)
	d.Set("qemu_os", config.QemuOs)

	if config.CIuser != "" {
		d.Set("cloudinit_user", config.CIuser)
	}
	if config.CIpassword != "" {
		d.Set("cloudinit_password", config.CIpassword)
	}
	if config.Searchdomain != "" {
		d.Set("cloudinit_searchdomain", config.Searchdomain)
	}
	if config.Nameserver != "" {
		d.Set("cloudinit_nameserver", config.Nameserver)
	}
	if config.Sshkeys != "" {
		d.Set("cloudinit_sshkeys", config.Sshkeys)
	}
	if config.Ipconfig0 != "" {
		d.Set("cloudinit_ipconfig0", config.Ipconfig0)
	}
	if config.Ipconfig1 != "" {
		d.Set("cloudinit_ipconfig1", config.Ipconfig1)
	}

	// // Disks.
	// configDisksSet := d.Get("disk").([]interface{})
	// activeDisksSet := updateDevicesSet(configDisksSet, config.QemuDisks)
	// d.Set("disk", activeDisksSet)
	// d.Set("disk", configDisksSet)
	// Networks.
	// configNetworksSet := d.Get("network").([]interface{})
	// activeNetworksSet := updateDevicesSet(configNetworksSet, config.QemuNetworks)
	// d.Set("network", activeNetworksSet)
	// d.Set("network", configNetworksSet)

	return
}

// func resourceVmQemuImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
// 	// TODO: research proper import
// 	err := resourceVmQemuRead(d, meta)
// 	return []*schema.ResourceData{d}, err
// }

func resourceVmQemuDelete(d *schema.ResourceData, meta interface{}) (err error) {
	pconf := meta.(*providerConfiguration)
	pmParallelBegin(pconf)
	defer pmParallelEnd(pconf)
	client := pconf.Client
	vmId, _ := strconv.Atoi(d.Id())
	vmr := pxapi.NewVmRef(vmId)
	_, err = client.StopVm(vmr)
	if err != nil {
		return
	}
	// give sometime to proxmox to catchup
	time.Sleep(2 * time.Second)
	_, err = client.DeleteVm(vmr)
	return
}

func resourceVmQemuExists(d *schema.ResourceData, meta interface{}) (exists bool, err error) {
	pconf := meta.(*providerConfiguration)
	pmParallelBegin(pconf)
	defer pmParallelEnd(pconf)
	client := pconf.Client
	vmId, _ := strconv.Atoi(d.Id())
	vmr := pxapi.NewVmRef(vmId)

	_, err = client.GetVmState(vmr)
	if err != nil {
		return false, nil
	}
	return true, nil
}


// Increase disk size if original disk was smaller than new disk.
func prepareDiskSize(
	client *pxapi.Client,
	vmr *pxapi.VmRef,
	diskConfMap pxapi.QemuDevices,
) error {
	clonedConfig, err := pxapi.NewConfigQemuFromApi(vmr, client)
	for diskID, diskConf := range diskConfMap {
		diskName := fmt.Sprintf("%v%v", diskConf["type"], diskID)

		diskSizeGB := diskConf["size"].(string)
		diskSize, _ := strconv.ParseFloat(strings.Trim(diskSizeGB, "G"), 64)
		if err != nil {
			return err
		}

		if _, diskExists := clonedConfig.QemuDisks[diskID]; !diskExists {
			return err
		}
		clonedDiskSizeGB := clonedConfig.QemuDisks[diskID]["size"].(string)
		clonedDiskSize, _ := strconv.ParseFloat(strings.Trim(clonedDiskSizeGB, "G"), 64)

		if err != nil {
			return err
		}

		diffSize := int(math.Ceil(diskSize - clonedDiskSize))
		if diskSize > clonedDiskSize {
			_, err := client.ResizeQemuDisk(vmr, diskName, diffSize)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Update schema.TypeSet with new values comes from Proxmox API.
// TODO: Maybe it's better to create a new Set instead add to current one.

// func updateDevicesSet(
// 	devicesSet *schema.Set,
// 	devicesMap pxapi.QemuDevices,
// ) *schema.Set {

// 	configDevicesMap := devicesSetToMap(devicesSet)
// 	activeDevicesMap := updateDevicesDefaults(devicesMap, configDevicesMap)

// 	for _, setConf := range devicesSet.List() {
// 		devicesSet.Remove(setConf)
// 		setConfMap := setConf.(map[string]interface{})
// 		deviceID := setConfMap["id"].(int)
// 		// Value type should be one of types allowed by Terraform schema types.
// 		for key, value := range activeDevicesMap[deviceID] {
// 			// This nested switch is used for nested config like in `net[n]`,
// 			// where Proxmox uses `key=<0|1>` in string" at the same time
// 			// a boolean could be used in ".tf" files.
// 			switch setConfMap[key].(type) {
// 			case bool:
// 				switch value.(type) {
// 				// If the key is bool and value is int (which comes from Proxmox API),
// 				// should be converted to bool (as in ".tf" conf).
// 				case int:
// 					sValue := strconv.Itoa(value.(int))
// 					bValue, err := strconv.ParseBool(sValue)
// 					if err == nil {
// 						setConfMap[key] = bValue
// 					}
// 				// If value is bool, which comes from Terraform conf, add it directly.
// 				case bool:
// 					setConfMap[key] = value
// 				}
// 			// Anything else will be added as it is.
// 			default:
// 				setConfMap[key] = value
// 			}
// 			devicesSet.Add(setConfMap)
// 		}
// 	}

// 	return devicesSet
// }

// Because default values are not stored in Proxmox, so the API returns only active values.
// So to prevent Terraform doing unnecessary diffs, this function reads default values
// from Terraform itself, and fill empty fields.

// func updateDevicesDefaults(
// 	activeDevicesMap pxapi.QemuDevices,
// 	configDevicesMap pxapi.QemuDevices,
// ) pxapi.QemuDevices {

// 	for deviceID, deviceConf := range configDevicesMap {
// 		if _, ok := activeDevicesMap[deviceID]; !ok {
// 			activeDevicesMap[deviceID] = configDevicesMap[deviceID]
// 		}
// 		for key, value := range deviceConf {
// 			if _, ok := activeDevicesMap[deviceID][key]; !ok {
// 				activeDevicesMap[deviceID][key] = value
// 			}
// 		}
// 	}
// 	return activeDevicesMap
// }