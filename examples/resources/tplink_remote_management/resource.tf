resource "tplink_remote_management" "example" {
  # Web management port (1-65535) used for remote access.
  http_port = "8080"

  # Remote management IP address.
  # Enter "255.255.255.255" to allow all remote PCs.
  remote_host = "255.255.255.255"
}
