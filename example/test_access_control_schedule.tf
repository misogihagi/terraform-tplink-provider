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

# Every day from 08:00 to 17:30
resource "tplink_access_control_schedule" "workhours" {
  description = "WorkHours"
  week_day    = "day"
  start_time  = 16 # 08:00
  end_time    = 35 # 17:30
}

# All day on weekends
resource "tplink_access_control_schedule" "weekend_only" {
  description = "WeekendOnly"
  week_day    = "week"
  start_time  = 0  # 00:00
  end_time    = 47 # 24:00
  week_days   = ["sat", "sun"]
}
