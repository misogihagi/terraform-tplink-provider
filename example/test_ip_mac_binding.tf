terraform {
  required_providers {
    tplink = {
      source  = "registry.terraform.io/misogihagi/tplink"
      version = "~> 0.1"
    }
  }
}

provider "tplink" {
  endpoint = "http://192.168.1.1"
  username = "admin"
  password = "admin"
}

# Enable the ARP binding feature (IP & MAC バインディング)
resource "tplink_ip_mac_binding" "arp" {
  enabled = true
}

# Bind the server's MAC address to a fixed IP
resource "tplink_ip_mac_binding_entry" "server" {
  mac_address = "AA-BB-CC-DD-EE-F1"
  ip_address  = "192.168.1.100"
  enabled     = true
}

# Entry without binding (registered but not enforced)
resource "tplink_ip_mac_binding_entry" "printer" {
  mac_address = "AA-BB-CC-DD-EE-F2"
  ip_address  = "192.168.1.101"
  enabled     = false
}
