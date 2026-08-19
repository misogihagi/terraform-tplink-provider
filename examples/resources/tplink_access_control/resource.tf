resource "tplink_access_control" "example" {
  # Whether to enable Internet Access Control.
  enabled = true

  # The default filtering rule.
  # "allow" -> packets not specified by filtering rules are allowed.
  # "deny"  -> packets not specified by filtering rules are denied.
  default_action = "deny"
}
