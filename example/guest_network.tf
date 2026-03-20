resource "tplink_guest_network" "example" {
  enabled           = true
  ssid              = "GuestNet"
  max_stations      = 8
  security_type     = "WPA/WPA2-Personal"
  wpa_version       = "WPA2-PSK"
  encryption        = "AES"
  password          = "guestpassword123"
  lan_access        = false
  isolation         = true
  bandwidth_control = false
}
