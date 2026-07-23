resource "tplink_basic_security" "main" {
  # SPI Firewall
  spi_firewall = true

  # VPN Passthrough
  pptp_passthrough  = true
  l2tp_passthrough  = true
  ipsec_passthrough = true

  # ALG (Application Layer Gateway)
  ftp_alg  = true
  tftp_alg = true
  h323_alg = true
  sip_alg  = true
  rtsp_alg = true
}
