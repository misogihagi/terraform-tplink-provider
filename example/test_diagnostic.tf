terraform {
  required_providers {
    tplink = {
      source = "registry.terraform.io/misogihagi/tplink"
    }
  }
}

provider "tplink" {
  endpoint = "http://192.168.1.1"
  username = "admin"
  password = "admin"
}

data "tplink_diagnostic" "ping" {
  address    = "8.8.8.8"
  tool       = "ping"
  ping_count = "4"
}

output "ping_results" {
  value = data.tplink_diagnostic.ping.results
}

data "tplink_diagnostic" "traceroute" {
  address           = "8.8.8.8"
  tool              = "traceroute"
  traceroute_max_ttl = "20"
}

output "traceroute_results" {
  value = data.tplink_diagnostic.traceroute.results
}
