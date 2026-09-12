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

resource "tplink_firmware_upgrade" "test" {
  firmware_file = "/path/to/firmware.bin"
}

output "firmware_version" {
  value = tplink_firmware_upgrade.test.firmware_version
}

output "hardware_version" {
  value = tplink_firmware_upgrade.test.hardware_version
}