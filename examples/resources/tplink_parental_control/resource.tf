resource "tplink_parental_control" "example" {
  # Enable or disable Parental Controls.
  enabled = true

  # MAC address of the parent's PC.
  parent_mac = "AA:BB:CC:DD:EE:FF"

  # MAC addresses of the PCs whose access is restricted (up to 4).
  managed_macs = ["11:22:33:44:55:66"]

  # Restriction schedule. Times are option values: 00:00-23:30 = 0-47 (start),
  # 00:30-24:00 = 0-47 (end).
  schedule_start = "8"  # 04:00
  schedule_end   = "20" # 10:30

  # Restrict only on selected weekdays.
  weekly    = true
  week_days = ["mon", "tue", "wed"]

  # Websites to block.
  blocked_urls = ["facebook.com", "youtube.com"]
}
