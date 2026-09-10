terraform {
  required_providers {
    tplink = {
      source = "registry.terraform.io/misogihagi/tplink"
    }
  }
}

provider "tplink" {
  endpoint = "http://192.168.1.1"
  username = "admin"
  password = "admin"
}

resource "tplink_time_setting" "test" {
  timezone   = "+09:00"
  year       = "2026"
  month      = "09"
  day        = "10"
  hour       = "15"
  minute     = "30"
  second     = "00"
  ntp_server1 = "ntp.nict.jp"
  ntp_server2 = "time.asia.apple.com"

  dst_enabled      = true
  dst_start_month   = "03"
  dst_start_week    = "5"
  dst_start_weekday = "7"
  dst_start_time    = "02:00:00"
  dst_end_month     = "11"
  dst_end_week      = "1"
  dst_end_weekday   = "7"
  dst_end_time      = "02:00:00"
}
