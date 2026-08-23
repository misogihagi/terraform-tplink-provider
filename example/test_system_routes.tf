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

# Read the System Routing Table (高度経路 > システム経路テーブル).
# This data source is read-only.
data "tplink_system_routes" "current" {}

output "system_routes" {
  description = "All entries in the system routing table."
  value       = data.tplink_system_routes.current.routes
}
