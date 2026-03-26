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

resource "tplink_dhcp_server" "test" {
  enabled    = true
  start_ip   = "192.168.1.100"
  end_ip     = "192.168.1.199"
  lease_time = 120
  gateway    = "192.168.1.1"
  dns1       = "8.8.8.8"
  dns2       = "8.8.4.4"
}
