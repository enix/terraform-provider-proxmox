# Proxmox 5.3 Terraform Provider & Provisioner plugin

This is a fork of Telmate's initial work. It is mainly focused on cloud-init based images.

## Working prototype

## Build

Requires https://github.com/enix/proxmox-api-go

```
go build -o terraform-provider-proxmox
cp terraform-provider-proxmox $GOPATH/bin
cp terraform-provider-proxmox $GOPATH/bin/terraform-provisioner-proxmox
```

Note: this plugin is both a provider and provisioner in one, which is why it needs to be in the $GOPATH/bin/ twice.

Recommended ISO builder https://github.com/Telmate/terraform-ubuntu-proxmox-iso


## Run

```
terraform apply
```

### Sample file

main.tf:
```
provider "proxmox" {
	pm_tls_insecure = true
}

/* Uses cloud-init options from Proxmox 5.2 */
resource "proxmox_vm_qemu" "cloudinit-test" {
	name = "tftest1.xyz.com"
	desc = "tf description"
	target_node = "proxmox1-xx"
	clone = "ci-ubuntu-template"
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
	cloud_init {
		user = "debian"
		password = "toto"
		ipconfig0 = "ip=10.0.2.99, gw=10.0.2.2"
		sshkeys = <<EOF
ssh-rsa AAAAB3NzaC1kj...key1
ssh-rsa AAAAB3NzaC1kj...key2
EOF
	}

	provisioner "remote-exec" {
		inline = [
			"ip a"
		]
	}
}
```
### Provider usage
You can start from either an ISO or clone an existing VM.

Optimally, you could create a VM resource you will use a clone base with an ISO, and make the rest of the VM resources depend on that base "template" and clone it.

Interesting parameters:
**preprovision** - to enable or disable internal pre-provisioning (e.g. if you already have another way to provision VMs). Conflicts with: `ssh_forward_ip`, `ssh_user`, `ssh_private_key`, `os_type`, `os_network_config`.
**os_type** - 
* cloud-init  - from Proxmox 5.2
* ubuntu -(https://github.com/Telmate/terraform-ubuntu-proxmox-iso)
* centos - (TODO: centos iso template)

**ssh_forward_ip** - should be the IP or hostname of the target node or bridge IP. This is where proxmox will create a port forward to your VM with via a user_net. (for pre-cloud-init provisioning)

### Cloud-Init

Cloud-init VMs must be cloned from a cloud-init ready template. 
See: https://pve.proxmox.com/wiki/Cloud-Init_Support

* ciuser - User name to change ssh keys and password for instead of the image’s configured default user.
* cipassword - Password to assign the user.
* searchdomain - Sets DNS search domains for a container.
* nameserver - Sets DNS server IP address for a container.
* sshkeys - public ssh keys, one per line
* ipconfig0 - [gw=<GatewayIPv4>] [,gw6=<GatewayIPv6>] [,ip=<IPv4Format/CIDR>] [,ip6=<IPv6Format/CIDR>]
* ipconfig1 - optional, same as ipconfig0 format

### Preprovision (internal alternative to Cloud-Init)

There is a pre-provision phase which is used to set a hostname, intialize eth0, and resize the VM disk to available space. This is done over SSH with the ssh_forward_ip, ssh_user and ssh_private_key.

Disk resize is done if the file /etc/auto_resize_vda.sh exists. Source: https://github.com/Telmate/terraform-ubuntu-proxmox-iso/blob/master/auto_resize_vda.sh

### Provisioner usage


Remove the temporary net1 adapter.
Inside the VM this usually triggers the routes back to the provisioning machine on net0.
```
	provisioner "proxmox" {
		action = "sshbackward"
	}

```

Replace the temporary net1 adapter with a new persistent net1.
```
	provisioner "proxmox" {
		action = "reconnect"
		net1 = "virtio,bridge=vmbr0,tag=99"
	}

```
If net1 needs a config other than DHCP you should prior to this use provisioner "remote-exec" to modify the network config.
