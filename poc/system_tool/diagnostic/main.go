package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
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
		Headless: playwright.Bool(false),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"

	tool := "ping" // "ping" or "traceroute"
	addr := "8.8.8.8"
	pingCount := "4"
	pingSize := "64"
	pingTimeout := "1"
	traceMaxTTL := "20"

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// Login
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

	// Find bottomLeftFrame
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

	// Click System Tools menu
	fmt.Println("Clicking System Tools menu...")
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Diagnostic submenu
	fmt.Println("Clicking Diagnostic submenu...")
	diagLoc := leftFrame.Locator("a:has-text('診断'), a:has-text('Diagnostic')").First()
	if err := diagLoc.WaitFor(); err == nil {
		_ = diagLoc.Click()
		time.Sleep(1 * time.Second)
	}

	// Find mainFrame
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

	// Wait for diagnostic page
	if err := mainFrame.Locator("#testButton").WaitFor(); err != nil {
		log.Fatalf("could not wait for diagnostic page: %v", err)
	}

	// Select tool
	fmt.Printf("Selecting tool: %s\n", tool)
	switch tool {
	case "ping":
		_ = mainFrame.Locator("#ipping").Check()
	case "traceroute":
		_ = mainFrame.Locator("#traceroute").Check()
	}
	time.Sleep(500 * time.Millisecond)

	// Fill address
	fmt.Printf("Setting address: %s\n", addr)
	_ = mainFrame.Locator("#l_addr").Fill(addr)

	// Fill tool-specific params
	if tool == "ping" {
		_ = mainFrame.Locator("#l_ping_pkt").Fill(pingCount)
		_ = mainFrame.Locator("#l_ping_pkt_size").Fill(pingSize)
		_ = mainFrame.Locator("#l_ping_pkt_time").Fill(pingTimeout)
	} else {
		_ = mainFrame.Locator("#l_tr_hop").Fill(traceMaxTTL)
	}

	// Click Start
	fmt.Println("Starting diagnostic...")
	_ = mainFrame.Locator("#testButton").Click()

	// Wait for results
	fmt.Println("Waiting for results...")
	time.Sleep(10 * time.Second)

	// Read results
	rows, err := mainFrame.Locator("table#display_table tr").All()
	if err != nil {
		log.Fatalf("could not read result rows: %v", err)
	}

	fmt.Println("\n=== Diagnostic Results ===")
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) == 0 {
			continue
		}
		line := strings.TrimSpace(strings.Join(cols, " | "))
		if line != "" {
			fmt.Println(line)
		}
	}

	fmt.Println("\nPoC Finished.")
}
