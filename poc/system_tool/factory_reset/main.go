package main

import (
	"fmt"
	"log"
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

	// Click Factory Defaults (工場出荷時の設定) submenu
	fmt.Println("Clicking Factory Defaults submenu...")
	restoreLoc := leftFrame.Locator("a:has-text('工場出荷時の設定'), a:has-text('Factory Defaults'), a:has-text('Factory Restore')").First()
	if err := restoreLoc.WaitFor(); err == nil {
		_ = restoreLoc.Click()
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

	// Wait for the factory defaults page
	fmt.Println("Waiting for factory defaults page...")
	if err := mainFrame.Locator("#t_restore").WaitFor(); err != nil {
		log.Fatalf("could not wait for factory defaults page: %v", err)
	}

	// Read warning text
	info, err := mainFrame.Locator("#t_info").InnerText()
	if err != nil {
		log.Printf("could not read info text: %v", err)
	} else {
		fmt.Printf("Info text: %s\n", info)
	}

	// Handle confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// Click Restore button
	fmt.Println("Clicking Restore button...")
	_ = mainFrame.Locator("#t_restore").Click()
	time.Sleep(1 * time.Second)

	// Some models prompt for the admin password before restoring
	if err := page.Locator("#pcPassword").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(3000),
	}); err == nil {
		_ = page.Locator("#pcPassword").Fill(password)
		_ = page.Locator("#confirmBtn").Click()
	}

	// Wait for router to restart and restore factory defaults
	fmt.Println("Waiting for router to restart (90s)...")
	time.Sleep(90 * time.Second)

	// Attempt to reconnect to verify the router is back (login page)
	fmt.Println("Attempting to verify the router is back online...")
	for i := 0; i < 6; i++ {
		if _, err := page.Goto(endpoint); err == nil {
			err := page.Locator("#userName").WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})
			if err == nil {
				fmt.Println("Login page is available. Factory reset PoC succeeded.")
				break
			}
		}
		time.Sleep(10 * time.Second)
	}

	fmt.Println("PoC Finished.")
}