# Proxmox 5.3 Terraform Provider

This is a fork of Telmate's initial work. It is mainly focused on cloud-init based images.

## Working prototype

## Build

Requires https://github.com/enix/proxmox-api-go

```
go build -o terraform-provider-proxmox
cp terraform-provider-proxmox $GOPATH/bin
cp terraform-provider-proxmox $GOPATH/bin/terraform-provisioner-proxmox
```
Recommended ISO builder https://github.com/Telmate/terraform-ubuntu-proxmox-iso


## Run

```
terraform apply
```

### Sample file

main.tf:
```
provider "proxmox" {
	api_url = "https://<host>:<port>/api2/json"
	api_username = "root@pam"
	api_password = "dummypasswd"
	api_tls_insecure = true
	api_parallel_clones = false	// true would lead to Proxmox not being able to grab a lock file (sometimes)
	api_parallel_resizes = true	// never had a problem with it
	parallel_resources = 4		// maximum parallel calls to Proxmox API
}

/* Uses cloud-init options from Proxmox 5.2 onwards */
resource "proxmox_vm_qemu" "myinstance" {
	name = "terraform.proxmox.enix.io"
	desc = "a description"
	target_node = "<proxmox_cluster_node_name>"
	clone = "<template_name_to_clone>"
	storage = "local"
	cores = 3
	sockets = 1
	memory = 2560
	network {
		model = "virtio"
		bridge = "vmbr0"
	}
	network {
		model = "virtio"
		bridge = "vmbr0"
	}
	disk {
		type = virtio
		storage = local-lvm
		storage_type = lvm
		size = 4G
		backup = true
	}
	cloudinit_password = "another dummy password"
	cloudinit_sshkeys = "${file(<some key path>)}"
	cloudinit_nameserver = "<ip1> <ip2>" // space separated list of ips
	cloudinit_searchdomain = "enix.io"

	cloudinit_ipconfig0 = "ip=<ip>,gw=<another_ip>"
	cloudinit_ipconfig1 = "ip=<ip>,gw=<another_ip>" // only two interfaces supported max by Proxmox cloudinit
	// comma separated values are listed in the /api2/json/nodes/{node}/qemu/{vmid}/config API call documentation under ipconfig[n]

	provisioner "remote-exec" {
		inline = [
			"ls"
		]
	}
}
```

### Cloud-Init

Cloud-init VMs must be cloned from a cloud-init ready template. 
See: https://pve.proxmox.com/wiki/Cloud-Init_Support

* cloudinit_user - User name to change ssh keys and password for instead of the image’s configured default user.
* cloudinit_password - Password to assign the user.
* cloudinit_searchdomain - Sets DNS search domains for a container.
* cloudinit_nameserver - Sets DNS server IP address for a container.
* cloudinit_sshkeys - public ssh keys, one per line
* cloudinit_ipconfig0 - [gw=<GatewayIPv4>] [,gw6=<GatewayIPv6>] [,ip=<IPv4Format/CIDR>] [,ip6=<IPv6Format/CIDR>]
* cloudinit_ipconfig1 - optional, same as ipconfig0 format