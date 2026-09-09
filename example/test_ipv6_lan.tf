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

# Configure the IPv6 LAN settings (IPv6 > IPv6 LAN 設定).
resource "tplink_ipv6_lan" "test" {
  address_config_type = "radvd" # "radvd" or "dhcp6s"
  prefix_config_type  = "delegated" # "delegated" or "static"

  # --- RADVD options ---
  rdnss_enabled = true
  ula_enabled   = false

  # --- DHCPv6 server options (when address_config_type = "dhcp6s") ---
  # min_interface_id = "1000"   # (1~FFFE)
  # max_interface_id = "2000"   # (1~FFFE)
  # lease_time       = "86400"  # seconds

  # --- Static prefix options (when prefix_config_type = "static") ---
  # site_prefix        = "2001:db8:1234:5678"
  # site_prefix_length = "64"
}