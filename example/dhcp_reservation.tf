resource "tplink_dhcp_reservation" "example" {
  mac_address = "AA:BB:CC:DD:EE:FF"
  ip_address  = "192.168.1.150"
  enabled     = true
}
