resource "tplink_wireless24" "example" {
  ssid           = "TP-Link_SSID_Terraform"
  mode           = "n"
  channel        = "0"
  channel_width  = "Auto"
  ssid_broadcast = true
}
