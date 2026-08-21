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

# IP mode host entry
resource "tplink_access_control_host" "pc1" {
  description = "MyPC"
  mode        = "ip"
  ip_start    = "192.168.1.100"
  ip_end      = "192.168.1.100"
  port_start  = "0"
  port_end    = "65535"
}

# MAC mode host entry
resource "tplink_access_control_host" "laptop1" {
  description = "MyLaptop"
  mode        = "mac"
  mac_address = "AA:BB:CC:DD:EE:FF"
}
