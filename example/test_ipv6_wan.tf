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

# Configure the IPv6 WAN settings (IPv6 > IPv6 WAN).
resource "tplink_ipv6_wan" "test" {
  wan_connection_enabled = true
  ipv6_enabled           = true
  connection_type        = "dynamicIp"

  dynamic_ipv4_enabled          = true
  dynamic_ipv6_enabled          = true
  dynamic_ipv6_addressing_type  = "dhcp" # "dhcp" or "autoip" (SLAAC)
  dynamic_prefix_delegation_only = false
  dynamic_mtu                   = "1500"
  dynamic_manual_dns6           = false
}

output "ipv6_wan_connection_type" {
  description = "The configured IPv6 WAN connection type."
  value       = tplink_ipv6_wan.test.connection_type
}