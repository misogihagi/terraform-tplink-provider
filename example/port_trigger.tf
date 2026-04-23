resource "tplink_port_trigger" "example" {
  trigger_port     = "1234"
  trigger_protocol = "TCP"
  open_port        = "5678"
  open_protocol    = "TCP"
  enabled          = true
}

resource "tplink_port_trigger" "range_example" {
  trigger_port     = "8000"
  trigger_protocol = "UDP"
  open_port        = "8000-8010"
  open_protocol    = "TCP or UDP"
  enabled          = true
}
