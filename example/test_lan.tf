terraform {
  required_providers {
    tplink = {
      source = "registry.terraform.io/misogihagi/tplink"
    }
  }
}

provider "tplink" {
  endpoint = "http://192.168.1.1" # Adjust to current router IP if needed
  username = "admin"
  password = "admin"
}

resource "tplink_lan" "my_lan" {
  ip_address = "192.168.11.1"
}
