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

# Read the IPv6 Status (IPv6 > IPv6 ステータス).
# This data source is read-only.
data "tplink_ipv6_status" "current" {}

output "ipv6_wan" {
  description = "The current WAN IPv6 status section."
  value       = data.tplink_ipv6_status.current.wan
}

output "ipv6_lan" {
  description = "The IPv6 LAN status section."
  value       = data.tplink_ipv6_status.current.lan
}