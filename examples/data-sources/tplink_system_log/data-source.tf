data "tplink_system_log" "example" {}

output "system_log" {
  value = data.tplink_system_log.example.entries
}