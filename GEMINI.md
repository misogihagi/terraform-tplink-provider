# TP-Link Terraform Provider (terraform-provider-tplink)

This is a custom Terraform provider to automate TP-Link router configurations.
The key characteristic is that it doesn't use a standard API. Instead, it uses **Playwright (`playwright-community/playwright-go`) to directly interact with (scrape) the router's web management interface (e.g., `192.168.1.1`) using a headless browser.**

## Tech Stack
- **Language**: Go
- **Terraform Framework**: Terraform Plugin Framework (`hashicorp/terraform-plugin-framework`)
- **Browser Automation**: Playwright for Go (`playwright-community/playwright-go`)

## Directory Structure
- `main.go`: Entry point for the provider.
- `internal/provider/`: Implementation of the provider and resources.
  - `provider.go`: Provider initialization and Playwright connection settings (Endpoint, Username, Password, etc.).
  - `lan_resource.go`: `tplink_lan` resource. Implements setting the router's LAN IP address.
  - `operation_mode_resource.go`: `tplink_operation_mode` resource. Implements setting the router's operation mode (Router, AP, etc.).
- `example/`: Terraform files for manual testing.
- `poc/`: PoC code running itself.


## Guidelines for AI Assistants (Gemini, etc.)
When adding features or modifying this repository, please keep the following in mind:

### 1. Browser Automation (Playwright) Considerations
- The TP-Link web UI heavily uses **frames** (e.g., `bottomLeftFrame`, `mainFrame`). When locating or interacting with elements, you must first obtain the correct frame instance before using a `Locator`.
- After navigating or saving settings, the router might implicitly reboot or reload. Include appropriate `Wait` logic (`page.WaitForLoadState`, `time.Sleep`, etc.) to ensure a stable script.
- Selectors might vary by device model or firmware version. Refer to existing code for common selectors (ID, name, text). Use robust selectors (e.g., combining `a:has-text` with IDs like `#a2`) to handle potential language variations in the UI.
- **Locator format**: Always prefer selectors that combine the **element type and ID** (e.g., `input#ipaddr`, `select#proto`, `button#save`). This format is both precise and resilient to UI text changes. Avoid using text-only selectors or overly broad CSS selectors when an ID is available.

### 2. Terraform Plugin Framework Conventions
- When adding resources, implement structs that satisfy `resource.Resource` and `resource.ResourceWithConfigure`.
- Due to the nature of web scraping, state management might be simplified (e.g., treating the value set during creation as the source of truth). `Read` operations are sometimes minimized to avoid the high overhead of scraping.

### 3. Debugging and Verification
- Use local builds for development. Override with `.terraform.rc` or manually point test files to the local provider binary.
- When an error occurs during browser interaction, ensure detailed error messages (from Playwright) are returned via `resp.Diagnostics.AddError`.
