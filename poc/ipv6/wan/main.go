package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// IPv6 WAN settings page (IPv6 > IPv6 WAN) PoC.
// This demonstrates:
//  1. Login to the router.
//  2. Navigate to IPv6 > IPv6 WAN.
//  3. Read the current WAN connection type (link_type).
//  4. Switch the connection type and fill in each sub-section
//     (passthrough / dynamicIp / staticIp / pppoe / 6to4).
//
// NOTE: this is a READ-ONLY PoC by default. Set doSave = true to actually
// click the 保存 (Save) button and commit the changes to the router.
func main() {
	// === Configuration Values ===
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"
	doSave := false

	// === Playwright Setup ===
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

	// Auto-accept confirmation dialogs (e.g. on save / reboot)
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		_ = dialog.Accept()
	})

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// === Login ===
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

	// === Navigate to IPv6 > IPv6 WAN ===
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

	fmt.Println("Clicking IPv6 menu...")
	ipv6MenuLoc := leftFrame.Locator("a:has-text('IPv6')").First()
	if err := ipv6MenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for IPv6 menu: %v", err)
	}
	_ = ipv6MenuLoc.Click()
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking IPv6 WAN submenu...")
	wanMenuLoc := leftFrame.Locator("a:has-text('IPv6 WAN'), a:has-text('IPv6 無線LAN')").First()
	if err := wanMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for IPv6 WAN submenu: %v", err)
	}
	_ = wanMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// === Read the page from mainFrame ===
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

	if err := mainFrame.Locator("p#et").WaitFor(); err != nil {
		log.Fatalf("could not wait for IPv6 WAN page to load: %v", err)
	}
	time.Sleep(1 * time.Second)

	// === Read current settings ===
	readCheck := func(selector string) bool {
		loc := mainFrame.Locator(selector).First()
		checked, err := loc.IsChecked()
		return err == nil && checked
	}

	fmt.Println("--- Current IPv6 WAN settings ---")
	fmt.Printf("  Enable WAN connection (ethWan_en): %v\n", readCheck("#ethWan_en"))
	fmt.Printf("  IPv6 を有効にする (wan_enable_ipv6): %v\n", readCheck("#wan_enable_ipv6"))

	linkType, _ := mainFrame.Locator("#link_type").InputValue()
	fmt.Printf("  接続タイプ (link_type): %s\n", linkType)

	// The page shows/hides different sub-sections depending on link_type.
	// Print which section is currently visible.
	for _, sec := range []struct{ id, name string }{
		{"#pass_through_ipv6_elem_adv", "パススルー"},
		{"#dyn_ip_elem_basic", "動的 IPv6"},
		{"#ip_elem_basic", "静的 IPv6"},
		{"#pppoe_elem_basic", "PPPoEv6"},
	} {
		loc := mainFrame.Locator(sec.id).First()
		visible, err := loc.IsVisible()
		fmt.Printf("  section %-28s visible=%v\n", sec.name, err == nil && visible)
	}

	// === Demonstrate switching connection type ===
	// Uncomment one of the following to demonstrate selecting a connection type.
	// These all call showLnkType() onchange automatically.
	target := "" // e.g. "passthrough", "dynamicIp", "staticIp", "pppoe", "6to4"

	switch target {
	case "passthrough":
		if _, err := mainFrame.Locator("#link_type").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice("passthrough"),
		}); err != nil {
			log.Fatalf("could not select passthrough: %v", err)
		}
	case "dynamicIp":
		if _, err := mainFrame.Locator("#link_type").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice("dynamicIp"),
		}); err != nil {
			log.Fatalf("could not select dynamicIp: %v", err)
		}
	case "staticIp":
		if _, err := mainFrame.Locator("#link_type").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice("staticIp"),
		}); err != nil {
			log.Fatalf("could not select staticIp: %v", err)
		}
	case "pppoe":
		if _, err := mainFrame.Locator("#link_type").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice("pppoe"),
		}); err != nil {
			log.Fatalf("could not select pppoe: %v", err)
		}
	case "6to4":
		if _, err := mainFrame.Locator("#link_type").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice("6to4"),
		}); err != nil {
			log.Fatalf("could not select 6to4: %v", err)
		}
	}

	if target != "" {
		time.Sleep(500 * time.Millisecond)

		// Example field fills for the dynamic IPv6 section.
		// Uncomment to demonstrate filling values.
		//
		// _ = mainFrame.Locator("#wan_enable_ipv6").Check()
		// if dynIPv6 := mainFrame.Locator("#dyn_ip6_elem_enable").First(); true {
		// 	_ = dynIPv6.Check()
		// }
		// _ = mainFrame.Locator("#dyn_ip6addr_type").SelectOption(playwright.SelectOptionValues{
		// 	Values: playwright.StringSlice("dhcp"),
		// })
		// if manualDns := mainFrame.Locator("#dynamic_manual_dns6").First(); true {
		// 	_ = manualDns.Check()
		// 	_ = mainFrame.Locator("#dyn_dns6_1").Fill("2001:4860:4860::8888")
		// 	_ = mainFrame.Locator("#dyn_dns6_2").Fill("2001:4860:4860::8844")
		// }

		fmt.Printf("Switched link_type to: %s\n", target)
	}

	// === Save ===
	if doSave {
		fmt.Println("Clicking 保存 (Save) button...")
		saveBtn := mainFrame.Locator("input#saveBtn").First()
		if err := saveBtn.WaitFor(); err != nil {
			log.Fatalf("could not wait for save button: %v", err)
		}
		_ = saveBtn.Click()
		time.Sleep(5 * time.Second)
		fmt.Println("Save clicked.")
	} else {
		fmt.Println("Read-only PoC: skipping save.")
	}

	fmt.Println("PoC Finished.")
}
