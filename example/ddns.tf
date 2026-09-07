terraform {
  required_providers {
    tplink = {
      source = "registry.terraform.io/misogihagi/tplink"
    }
  }
}

provider "tplink" {
  endpoint = "http://192.168.1.1" # Change to your router's IP
  username = "admin"           # Change to your router's username
  password = "admin"           # Change to your router's password
}

resource "tplink_ddns" "test" {
  provider      = "noipDns"
  noip_domain   = "example.ddns.net"
  username      = "my-dyndns-user"
  password      = "my-dyndns-pass"
  wan_ip_binding = true
  enabled       = true
  login         = true
}