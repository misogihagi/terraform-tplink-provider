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

resource "tplink_password_settings" "test" {
  # Current credentials used to authenticate the change.
  # These must be the credentials the router is currently using.
  old_username = "admin"
  old_password = "admin"

  # New credentials to apply.
  # Username and password must be 1 to 15 characters with no spaces.
  new_username     = "admin2"
  new_password     = "admin2"
  confirm_password = "admin2"
}

output "id" {
  value = tplink_password_settings.test.id
}