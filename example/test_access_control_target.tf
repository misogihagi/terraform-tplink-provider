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

# IP mode target entry
resource "tplink_access_control_target" "local_net" {
  description = "LocalNet"
  mode        = "ip"
  ip_start    = "192.168.1.0"
  ip_end      = "192.168.1.255"
  port_start  = "80"
  port_end    = "443"
}

# MAC mode target entry
resource "tplink_access_control_target" "printer" {
  description = "Printer"
  mode        = "mac"
  mac_address = "AA:BB:CC:DD:EE:FF"
}

# URL mode target entry
resource "tplink_access_control_target" "social_media" {
  description = "SocialMedia"
  mode        = "url"
  urls        = ["facebook.com", "twitter.com"]
}
