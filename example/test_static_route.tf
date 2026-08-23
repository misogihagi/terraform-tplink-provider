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

# Route via the Internet connection (enabled by default)
resource "tplink_static_route" "vpn_net" {
  destination = "10.0.0.0"
  netmask     = "255.0.0.0"
  gateway     = "192.168.1.254"
  interface   = "Internet"
}

# Route via LAN, explicitly disabled
resource "tplink_static_route" "lab_net" {
  destination = "172.16.0.0"
  netmask     = "255.240.0.0"
  gateway     = "192.168.1.253"
  interface   = "LAN"
  enabled     = false
}
