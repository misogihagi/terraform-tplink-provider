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

	// DHCP Address Reservation Settings
	macAddr := "AA:BB:CC:DD:EE:FF"
	ipAddr := "192.168.1.150"
	status := "1" // 1: Enabled, 0: Disabled

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
	dhcpMenuLoc := leftFrame.Locator("a:has-text('DHCP')").First()
	if err := dhcpMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for DHCP menu: %v", err)
	}
	_ = dhcpMenuLoc.Click()
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Address Reservation submenu...")
	reservationMenuLoc := leftFrame.Locator("a:has-text('アドレス予約'), a:has-text('Address Reservation')").First()
	if err := reservationMenuLoc.WaitFor(); err == nil {
		_ = reservationMenuLoc.Click()
		time.Sleep(1 * time.Second)
	}

	// 4. Interaction (mainFrame)
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

	// Wait for the list page to load
	fmt.Println("Clicking Add New button...")
	addNewBtn := mainFrame.Locator("input.T_addnew").First()
	if err := addNewBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for Add New button: %v", err)
	}
	_ = addNewBtn.Click()
	time.Sleep(1 * time.Second)

	// Wait for the add page to load (checking for macAddr input)
	fmt.Println("Filling reservation details...")
	macInput := mainFrame.Locator("input#macAddr")
	if err := macInput.WaitFor(); err != nil {
		log.Fatalf("could not wait for MAC address input: %v", err)
	}
	_ = macInput.Fill(macAddr)
	_ = mainFrame.Locator("input#ipAddr").Fill(ipAddr)
	
	fmt.Printf("Setting status to: %s\n", status)
	_, _ = mainFrame.Locator("select#state").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(status),
	})

	// 5. Save
	fmt.Println("Saving settings...")
	submitBtn := mainFrame.Locator("input#submitBtn").First()
	if err := submitBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for save button: %v", err)
	}
	
	// Setup dialog handler
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	_ = submitBtn.Click()

	fmt.Println("Waiting for router to apply (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished.")
}
