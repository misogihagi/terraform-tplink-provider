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

resource "tplink_reboot" "test" {}

output "completed_at" {
  value = tplink_reboot.test.completed_at
}