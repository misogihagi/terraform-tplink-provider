package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
	// 1. Playwright Setup
	err := playwright.Install()
	if err != nil {
		log.Fatalf("could not install playwright: %v", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(false), // Show browser for PoC visibility
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	// === Configuration Values ===
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"

	// DHCP Settings
	dhcpEnable := true
	startIP := "192.168.1.100"
	endIP := "192.168.1.199"
	leaseTime := "120"
	gateway := "192.168.1.1"
	domain := ""
	dns1 := "8.8.8.8"
	dns2 := "8.8.4.4"

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 2. Login
	fmt.Println("Attempting login...")
	if err := page.Locator("#userName").WaitFor(); err != nil {
		log.Fatalf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(username)
	_ = page.Locator("#pcPassword").Fill(password)
	_ = page.Locator("#loginBtn").Click()

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// 3. Navigate to DHCP menu
	fmt.Println("Searching for bottomLeftFrame...")
	var leftFrame playwright.Frame
	for i := 0; i < 10; i++ {
		for _, f := range page.Frames() {
			if f.Name() == "bottomLeftFrame" {
				leftFrame = f
				break
			}
		}
		if leftFrame != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if leftFrame == nil {
		log.Fatal("could not find bottomLeftFrame")
	}

	fmt.Println("Clicking DHCP menu...")
	// Assuming there is a DHCP menu item in the left frame
	dhcpMenuLoc := leftFrame.Locator("a:has-text('DHCP')").First()
	if err := dhcpMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for DHCP menu: %v", err)
	}
	_ = dhcpMenuLoc.Click()
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking DHCP Settings submenu...")
	dhcpSettingsMenuLoc := leftFrame.Locator("a:has-text('DHCP 設定'), a:has-text('DHCP Settings')").First()
	if err := dhcpSettingsMenuLoc.WaitFor(); err == nil {
		_ = dhcpSettingsMenuLoc.Click()
		time.Sleep(1 * time.Second)
	}

	// 4. entering settings (mainFrame)
	fmt.Println("Searching for mainFrame...")
	var mainFrame playwright.Frame
	for i := 0; i < 10; i++ {
		for _, f := range page.Frames() {
			if f.Name() == "mainFrame" {
				mainFrame = f
				break
			}
		}
		if mainFrame != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if mainFrame == nil {
		log.Fatal("could not find mainFrame")
	}

	// Wait for the page to render by checking one of the inputs
	if err := mainFrame.Locator("input#ip1").WaitFor(); err != nil {
		log.Fatalf("could not wait for DHCP page to load: %v", err)
	}

	// DHCP Server Enable/Disable
	if dhcpEnable {
		fmt.Println("Enabling DHCP Server...")
		_ = mainFrame.Locator("input#dhcp_enable").Check()
	} else {
		fmt.Println("Disabling DHCP Server...")
		_ = mainFrame.Locator("input#dhcp_disable").Check()
	}

	// IP Range
	fmt.Printf("Setting IP Range: %s - %s\n", startIP, endIP)
	_ = mainFrame.Locator("input#ip1").Fill(startIP)
	_ = mainFrame.Locator("input#ip2").Fill(endIP)

	// Lease Time
	fmt.Printf("Setting Lease Time: %s\n", leaseTime)
	_ = mainFrame.Locator("input#lease").Fill(leaseTime)

	// Default Gateway
	if gateway != "" {
		fmt.Printf("Setting Default Gateway: %s\n", gateway)
		_ = mainFrame.Locator("input#gateway").Fill(gateway)
	}

	// Default Domain
	if domain != "" {
		fmt.Printf("Setting Default Domain: %s\n", domain)
		_ = mainFrame.Locator("input#domain").Fill(domain)
	}

	// Primary DNS
	if dns1 != "" {
		fmt.Printf("Setting Primary DNS: %s\n", dns1)
		_ = mainFrame.Locator("input#dnsserver1").Fill(dns1)
	}

	// Secondary DNS
	if dns2 != "" {
		fmt.Printf("Setting Secondary DNS: %s\n", dns2)
		_ = mainFrame.Locator("input#dnsserver2").Fill(dns2)
	}

	// 5. Save
	fmt.Println("Saving settings...")
	// Setup dialog handler in case the router prompts for restarting/rebooting
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	fmt.Println("Waiting for router to apply (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished.")
}
