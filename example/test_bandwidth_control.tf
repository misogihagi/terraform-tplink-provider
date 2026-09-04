terraform {
  required_providers {
    tplink = {
      source  = "registry.terraform.io/misogihagi/tplink"
      version = "~> 0.1"
    }
  }
}

provider "tplink" {
  endpoint = "http://192.168.1.1"
  username = "admin"
  password = "admin"
}

# Global Bandwidth Control settings (帯域幅制御)
resource "tplink_bandwidth_control" "settings" {
  enabled            = true
  link_type          = "other"
  upload_bandwidth   = 100000
  download_bandwidth = 200000
}

# Limit bandwidth for the lab network (192.168.1.100 - 150)
resource "tplink_bandwidth_control_rule" "lab_net" {
  ip_start    = "192.168.1.100"
  ip_end      = "192.168.1.150"
  protocol    = "all"
  priority    = 5
  up_min_bw   = 1024
  up_max_bw   = 50000
  down_min_bw = 2048
  down_max_bw = 100000
}

# TCP-only rule for guests with lower priority
resource "tplink_bandwidth_control_rule" "guests" {
  ip_start    = "192.168.1.200"
  ip_end      = "192.168.1.220"
  protocol    = "tcp"
  priority    = 8
  up_max_bw   = 10000
  down_max_bw = 20000
}
