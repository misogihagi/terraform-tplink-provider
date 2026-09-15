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

# Read the System Log (システムツール > システム ログ)
# This data source is read-only.
data "tplink_system_log" "current" {}

output "system_log" {
  description = "All entries in the system log."
  value       = data.tplink_system_log.current.entries
}