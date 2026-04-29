package main

import (
	"fmt"
	"log"
	"strings"
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

	// 3. Navigate to Forwarding -> UPnP
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

	fmt.Println("Clicking Forwarding menu...")
	forwardingMenuLoc := leftFrame.Locator("a:has-text('転送'), a:has-text('Forwarding')").First()
	if err := forwardingMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Forwarding menu: %v", err)
	}
	_ = forwardingMenuLoc.Click()
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking UPnP submenu...")
	upnpMenuLoc := leftFrame.Locator("a:has-text('UPnP')").First()
	if err := upnpMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for UPnP menu: %v", err)
	}
	_ = upnpMenuLoc.Click()
	time.Sleep(1 * time.Second)

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

	// Wait for the UPnP page to load
	fmt.Println("Checking UPnP status...")
	upnpEnIndicator := mainFrame.Locator("b#upnp_en")
	if err := upnpEnIndicator.WaitFor(); err != nil {
		log.Fatalf("could not wait for UPnP enable indicator: %v", err)
	}

	class, _ := upnpEnIndicator.GetAttribute("class")
	isEnabled := !strings.Contains(class, "nd")

	if isEnabled {
		fmt.Println("UPnP is currently ENABLED. Toggling to DISABLED...")
		_ = mainFrame.Locator("input#disBtn").Click()
	} else {
		fmt.Println("UPnP is currently DISABLED. Toggling to ENABLED...")
		_ = mainFrame.Locator("input#enBtn").Click()
	}

	fmt.Println("Waiting for router to apply (3s)...")
	time.Sleep(3 * time.Second)

	// Verify status after toggle
	class, _ = upnpEnIndicator.GetAttribute("class")
	isEnabled = !strings.Contains(class, "nd")
	fmt.Printf("UPnP status is now: %v\n", isEnabled)

	// 5. Read UPnP List
	fmt.Println("Reading UPnP List...")
	rows, _ := mainFrame.Locator("table#upnpTbl tr").All()
	if len(rows) == 0 {
		fmt.Println("UPnP List is empty.")
	} else {
		fmt.Println("ID\tDescription\tExternal Port\tProtocol\tInternal Port\tIP Address\tStatus")
		for _, row := range rows {
			cols, _ := row.Locator("td").AllInnerTexts()
			if len(cols) > 0 {
				fmt.Println(strings.Join(cols, "\t"))
			}
		}
	}

	fmt.Println("PoC Finished.")
}
