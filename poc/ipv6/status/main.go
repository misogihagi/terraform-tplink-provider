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

	// 3. Navigate to IPv6 > IPv6 Status menu
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

	fmt.Println("Clicking IPv6 Status submenu...")
	statusMenuLoc := leftFrame.Locator("a:has-text('IPv6 ステータス'), a:has-text('IPv6 Status')").First()
	if err := statusMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for IPv6 Status submenu: %v", err)
	}
	_ = statusMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// 4. Read status from mainFrame
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

	// Wait for the page to render by checking the page title element
	if err := mainFrame.Locator("p#et").WaitFor(); err != nil {
		log.Fatalf("could not wait for IPv6 Status page to load: %v", err)
	}
	time.Sleep(1 * time.Second)

	// helper to read span text (empty if element is hidden)
	readText := func(selector string) string {
		loc := mainFrame.Locator(selector).First()
		visible, err := loc.IsVisible()
		if err != nil || !visible {
			return ""
		}
		text, err := loc.TextContent()
		if err != nil {
			return ""
		}
		return text
	}

	// --- WAN section (main, if visible) ---
	if v, _ := mainFrame.Locator("#ip6_wan_status").IsVisible(); v {
		fmt.Println("WAN:")
		fmt.Printf("  インターフェイス名:   %s\n", readText("#interName"))
		fmt.Printf("  接続タイプ:           %s\n", readText("#connType"))
		fmt.Printf("  接続ステータス:       %s\n", readText("#connStatus"))
		fmt.Printf("  IPv6 アドレス:        %s\n", readText("#ipv6Addr"))
		fmt.Printf("  IPv6 デフォルトGW:    %s\n", readText("#ipv6Gateway"))
		fmt.Printf("  プライマリ IPv6 DNS:  %s\n", readText("#ipv6PriDns"))
		fmt.Printf("  セカンダリ IPv6 DNS:  %s\n", readText("#ipv6SecDns"))
	}

	// --- WAN tunnel section (6to4/6rd tunnel) ---
	if v, _ := mainFrame.Locator("#ip6_wan_tunnel_status").IsVisible(); v {
		fmt.Println("WAN (Tunnel):")
		fmt.Printf("  接続タイプ:           %s\n", readText("#tunnelConnType"))
		fmt.Printf("  関連インターフェイス: %s\n", readText("#tunnelInterName"))
	}

	// --- WAN disabled section ---
	if v, _ := mainFrame.Locator("#ip6_wan_disabled_status").IsVisible(); v {
		fmt.Println("WAN (Disabled):")
		fmt.Printf("  接続タイプ:           %s\n", readText("#disabledConnType"))
	}

	// --- WAN passthrough section (visible in the provided HTML) ---
	if v, _ := mainFrame.Locator("#ip6_wan_passthrough_status").IsVisible(); v {
		fmt.Println("WAN (Passthrough):")
		fmt.Printf("  接続タイプ:           %s\n", readText("#passthroughType"))
		fmt.Printf("  接続ステータス:       %s\n", readText("#passthroughConnStatus"))
	}

	// --- IPv6 LAN section ---
	if v, _ := mainFrame.Locator("#ip6_lan_status").IsVisible(); v {
		fmt.Println("IPv6 LAN:")
		fmt.Printf("  IPv6 アドレス タイプ: %s\n", readText("#cfgtype"))
		fmt.Printf("  接頭辞の長さ:         %s\n", readText("#lan6prelen"))
		fmt.Printf("  IPv6 アドレス:        %s\n", readText("#lan6addr"))
	}

	fmt.Println("PoC Finished.")
}