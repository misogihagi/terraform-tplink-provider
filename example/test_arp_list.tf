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

# Read the ARP list (IP & MAC バインディング > ARP リスト)
data "tplink_arp_list" "current" {}

output "arp_entries" {
  value = data.tplink_arp_list.current.entries
}
