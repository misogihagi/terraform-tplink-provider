package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// SystemRoute represents a single entry read from the System Routing Table
// (システム経路テーブル). This page is read-only; it displays the router's
// current system routing information.
type SystemRoute struct {
	// ID is the row number displayed in the table.
	ID string

	// DestinationNetwork is the destination network address (宛先 ネットワーク).
	DestinationNetwork string

	// SubnetMask is the subnet mask (サブネット マスク).
	SubnetMask string

	// Gateway is the gateway IP address (ゲートウェイ).
	Gateway string

	// Interface is the outbound interface (インターフェイス), e.g. "LAN & WLAN".
	Interface string
}

func main() {
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"

	// 1. Initialize Playwright
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
		Headless: playwright.Bool(false), // Set to false to see the interaction for PoC
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	// 2. Browser Context
	context, err := browser.NewContext()
	if err != nil {
		log.Fatalf("could not create context: %v", err)
	}

	page, err := context.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 3. Handle login
	fmt.Println("Waiting for login elements...")
	if err := page.Locator("#userName").WaitFor(); err != nil {
		log.Printf("wait for username error (maybe already logged in): %v", err)
	} else {
		_ = page.Locator("#userName").Fill(username)
		_ = page.Locator("#pcPassword").Fill(password)
		_ = page.Locator("#loginBtn").Click()
	}

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// 4. Find bottomLeftFrame and navigate to Advanced Routing > System Routing Table
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

	fmt.Println("Clicking Advanced Routing menu...")
	routingLoc := leftFrame.Locator("a:has-text('高度な経路'), a:has-text('Advanced Routing')").First()
	if err := routingLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Advanced Routing link: %v", err)
	}
	if err := routingLoc.Click(); err != nil {
		log.Fatalf("could not click Advanced Routing menu: %v", err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Clicking System Routing Table submenu...")
	systemLoc := leftFrame.Locator("a:has-text('システム経路テーブル'), a:has-text('System Routing')").First()
	if err := systemLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for System Routing Table link: %v", err)
	}
	if err := systemLoc.Click(); err != nil {
		log.Fatalf("could not click System Routing Table submenu: %v", err)
	}

	// 5. Find mainFrame and read the routing table (read-only)
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

	fmt.Println("Waiting for system route table...")
	if err := mainFrame.Locator("div.tbody table#log_tbl").WaitFor(); err != nil {
		log.Printf("wait for system route table: %v", err)
	}

	time.Sleep(1 * time.Second)

	// 6. Read all rows from the System Routing Table
	fmt.Println("Reading System Routing Table...")
	rows, err := mainFrame.Locator("table#log_tbl tr").All()
	if err != nil {
		log.Fatalf("could not read table rows: %v", err)
	}

	routes := make([]SystemRoute, 0, len(rows))
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 5 {
			continue // Skip header/invalid rows
		}

		routes = append(routes, SystemRoute{
			ID:                 strings.TrimSpace(cols[0]),
			DestinationNetwork: strings.TrimSpace(cols[1]),
			SubnetMask:         strings.TrimSpace(cols[2]),
			Gateway:            strings.TrimSpace(cols[3]),
			Interface:          strings.TrimSpace(cols[4]),
		})
	}

	if len(routes) == 0 {
		fmt.Println("System Routing Table is empty.")
	} else {
		fmt.Println("ID\tDestination Network\tSubnet Mask\tGateway\tInterface")
		for _, r := range routes {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
				r.ID, r.DestinationNetwork, r.SubnetMask, r.Gateway, r.Interface)
		}
	}

	fmt.Printf("PoC Finished. Read %d system route entries.\n", len(routes))
}
