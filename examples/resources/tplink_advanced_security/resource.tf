resource "tplink_advanced_security" "example" {
  # DoS Protection master toggle.
  # When set to false, the flood filter settings below are ignored by the router.
  dos_protection = true

  # ICMP-Flood attack filtering (valid threshold range: 5–3600 packets/sec)
  icmp_flood_filter = true
  icmp_threshold    = "50"

  # UDP-Flood attack filtering (valid threshold range: 5–3600 packets/sec)
  udp_flood_filter = true
  udp_threshold    = "500"

  # TCP-SYN-Flood attack filtering (valid threshold range: 5–3600 packets/sec)
  syn_flood_filter = true
  syn_threshold    = "30"

  # Block Ping packets from the WAN port (recommended for security)
  wan_ping_filter = true

  # Block Ping packets from the LAN port (set to false to allow LAN Ping)
  lan_ping_filter = false
}
