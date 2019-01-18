package proxmox

import (
	pxapi "github.com/enix/proxmox-api-go/proxmox"
	"github.com/hashicorp/terraform/helper/schema"
)

func state2ConfigQemu(d *schema.ResourceData) (config *pxapi.ConfigQemu, err error) {
	config = &pxapi.ConfigQemu{
		Name:			d.Get("name").(string),
		Description:	d.Get("desc").(string),
		Onboot:			d.Get("onboot").(bool),
		Memory:			d.Get("memory").(int),
		QemuCores:		d.Get("cores").(int),
		QemuSockets:	d.Get("sockets").(int),
		QemuOs:			d.Get("qemu_os").(string),
		QemuNetworks:	devicesList2QemuDevices(d.Get("network").([]interface{})),
		QemuDisks:		devicesList2QemuDevices(d.Get("disk").([]interface{})),
	}

	if cloudInitUser := d.Get("cloudinit_user").(string); cloudInitUser != "" {
		config.CIuser = cloudInitUser
	}
	if cloudInitPassword := d.Get("cloudinit_password").(string); cloudInitPassword != "" {
		config.CIpassword = cloudInitPassword
	}
	if cloudInitSearchdomain := d.Get("cloudinit_searchdomain").(string); cloudInitSearchdomain != "" {
		config.Searchdomain = cloudInitSearchdomain
	}
	if cloudInitNameserver := d.Get("cloudinit_nameserver").(string); cloudInitNameserver != "" {
		config.Nameserver = cloudInitNameserver
	}
	if cloudInitSshkeys := d.Get("cloudinit_sshkeys").(string); cloudInitSshkeys != "" {
		config.Sshkeys = cloudInitSshkeys
	}
	if cloudInitIpconfig0 := d.Get("cloudinit_ipconfig0").(string); cloudInitIpconfig0 != "" {
		config.Ipconfig0 = cloudInitIpconfig0
	}
	if cloudInitIpconfig1 := d.Get("cloudinit_ipconfig1").(string); cloudInitIpconfig1 != "" {
		config.Ipconfig1 = cloudInitIpconfig1
	}
	return
}

// func devicesSetToMap(devicesSet *schema.Set) pxapi.QemuDevices {

// 	devicesMap := pxapi.QemuDevices{}

// 	for _, set := range devicesSet.List() {
// 		setMap, isMap := set.(map[string]interface{})
// 		if isMap {
// 			setID := setMap["id"].(int)
// 			devicesMap[setID] = setMap
// 		}
// 	}
// 	return devicesMap
// }

func devicesList2QemuDevices(devicesList []interface{}) pxapi.QemuDevices {

	qemuDevices := pxapi.QemuDevices{}

	for deviceID, set := range devicesList {
		qemuDevices[deviceID] = set.(map[string]interface{})
	}
	return qemuDevices
}