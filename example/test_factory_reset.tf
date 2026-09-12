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

resource "tplink_factory_reset" "test" {
  # Admin password for re-confirmation (optional, required on some models)
  admin_password = "admin"
}

output "completed_at" {
  value = tplink_factory_reset.test.completed_at
}