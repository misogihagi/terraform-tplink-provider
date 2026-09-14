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

resource "tplink_backup_restore" "test" {
  # Save the current configuration to a local file
  backup_file = "/tmp/router-config.bin"

  # Optionally restore from a previously saved configuration file
  # restore_file = "/tmp/previous-config.bin"
  # admin_password = "admin"
}

output "backup_completed_at" {
  value = tplink_backup_restore.test.backup_completed_at
}