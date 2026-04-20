resource "tplink_virtual_server" "example" {
  service_port  = "8080"
  ip_address    = "192.168.1.100"
  internal_port = "80"
  protocol      = "TCP"
  enabled       = true
}

resource "tplink_virtual_server" "range_example" {
  service_port  = "9000-9005"
  ip_address    = "192.168.1.101"
  protocol      = "TCP or UDP"
  enabled       = true
}
