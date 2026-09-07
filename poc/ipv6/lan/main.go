package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// IPv6 LAN settings page (IPv6 > IPv6 LAN 設定) PoC.
// This demonstrates:
//  1. Login to the router.
//  2. Navigate to IPv6 > IPv6 LAN 設定.
//  3. Read the current settings:
//       - アドレス自動設定タイプ (RADVD / DHCPv6 サーバー)
//       - RADVD options (RDNSS, ULA)
//       - サイト 接頭辞設定タイプ (委任 / 静的)
//  4. Switch address mode and prefix mode, and fill in each sub-section.
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

	// === Navigate to IPv6 > IPv6 LAN 設定 ===
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

	fmt.Println("Clicking IPv6 LAN submenu...")
	lanMenuLoc := leftFrame.Locator("a:has-text('IPv6 LAN')").First()
	if err := lanMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for IPv6 LAN submenu: %v", err)
	}
	_ = lanMenuLoc.Click()
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
		log.Fatalf("could not wait for IPv6 LAN page to load: %v", err)
	}
	time.Sleep(1 * time.Second)

	// === Read current settings ===
	readChecked := func(selector string) bool {
		loc := mainFrame.Locator(selector).First()
		checked, err := loc.IsChecked()
		return err == nil && checked
	}
	readVisible := func(selector string) bool {
		loc := mainFrame.Locator(selector).First()
		visible, err := loc.IsVisible()
		return err == nil && visible
	}
	readText := func(selector string) string {
		loc := mainFrame.Locator(selector).First()
		text, err := loc.InputValue()
		if err != nil {
			return ""
		}
		return text
	}

	fmt.Println("--- Current IPv6 LAN settings ---")
	groupText, _ := mainFrame.Locator("#brname").First().TextContent()
	fmt.Printf("  グループ (brname): %s\n", groupText)

	if readChecked("#radvd_en") {
		fmt.Println("  アドレス自動設定タイプ: RADVD")
	} else if readChecked("#dhcp6s_en") {
		fmt.Println("  アドレス自動設定タイプ: DHCPv6 サーバー")
	} else {
		fmt.Println("  アドレス自動設定タイプ: (none)")
	}

	if readVisible("#radvd_opt") {
		fmt.Println("  RADVD options:")
		fmt.Printf("    RDNSS を有効にする (rdnss_en):  %v\n", readChecked("#rdnss_en"))
		fmt.Printf("    ULA 接頭辞を有効にする (ula_en): %v\n", readChecked("#ula_en"))
		if readVisible("#ula_opt") {
			fmt.Printf("    ULA 接頭辞 (ula_pfx):          %s\n", readText("#ula_pfx"))
			fmt.Printf("    ULA 接頭辞の長さ (ula_plen):   %s\n", readText("#ula_plen"))
		}
	}

	if readVisible("#dhcp6s_opt") {
		fmt.Println("  DHCPv6 server options:")
		start, _ := mainFrame.Locator("#start_pfx").First().TextContent()
		fmt.Printf("    開始 IPv6 アドレス (min_intf_id): %s + %s (1~FFFE)\n", start, readText("#min_intf_id"))
		end, _ := mainFrame.Locator("#end_pfx").First().TextContent()
		fmt.Printf("    終了 IPv6 アドレス (max_intf_id): %s + %s (1~FFFE)\n", end, readText("#max_intf_id"))
		fmt.Printf("    リース期間 (ls_time):             %s 秒\n", readText("#ls_time"))
	}

	if readChecked("#pfx_delegated") {
		fmt.Println("  サイト接頭辞設定タイプ: 委任 (delegated)")
		curWan, _ := mainFrame.Locator("#curwaninf").First().TextContent()
		fmt.Printf("    接頭辞委任WAN接続 (curwaninf): %s\n", curWan)
	} else if readChecked("#pfx_static") {
		fmt.Println("  サイト接頭辞設定タイプ: 静的 (static)")
		fmt.Printf("    サイト接頭辞 (site_pfx):        %s\n", readText("#site_pfx"))
		fmt.Printf("    サイト接頭辞の長さ (site_plen): %s\n", readText("#site_plen"))
	} else {
		fmt.Println("  サイト接頭辞設定タイプ: (none)")
	}

	// === Demonstrate switching modes ===
	// Uncomment one of the following to demonstrate selecting a mode.
	demoMode := "" // e.g. "dhcp6s", "static"

	switch demoMode {
	case "dhcp6s":
		// Switch to DHCPv6 サーバー (address auto-config type)
		_ = mainFrame.Locator("input#dhcp6s_en").Check()
		time.Sleep(500 * time.Millisecond)
		// Example fills:
		// _ = mainFrame.Locator("#min_intf_id").Fill("1000")
		// _ = mainFrame.Locator("#max_intf_id").Fill("2000")
		// _ = mainFrame.Locator("#ls_time").Fill("86400")
		fmt.Println("Switched addrMode to DHCPv6 サーバー")
	case "static":
		// Switch to 静的 (static site prefix)
		_ = mainFrame.Locator("input#pfx_static").Check()
		time.Sleep(500 * time.Millisecond)
		// Example fills:
		// _ = mainFrame.Locator("#site_pfx").Fill("2001:db8:1234:5678")
		// _ = mainFrame.Locator("#site_plen").Fill("64")
		fmt.Println("Switched pfxMode to 静的 (static)")
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