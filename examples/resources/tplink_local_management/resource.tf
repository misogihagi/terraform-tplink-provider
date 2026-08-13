resource "tplink_local_management" "example" {
  # Management rule.
  # true  -> "すべて" (All): all PCs on the LAN can access the router's Web-based utility.
  # false -> "のみ" (Only): only the PC with the MAC address below can access.
  allow_all = false

  # The MAC address of the only PC allowed to manage.
  # Only applied when allow_all is false.
  mac_address = "AA:BB:CC:DD:EE:FF"
}
