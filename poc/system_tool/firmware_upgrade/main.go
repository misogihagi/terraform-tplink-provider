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
	firmwarePath := "/path/to/firmware.bin"

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

	// Click Firmware Upgrade submenu
	fmt.Println("Clicking Firmware Upgrade submenu...")
	fwLoc := leftFrame.Locator("a:has-text('ファームウェア アップグレード'), a:has-text('Firmware Upgrade')").First()
	if err := fwLoc.WaitFor(); err == nil {
		_ = fwLoc.Click()
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

	// Wait for firmware upgrade page
	if err := mainFrame.Locator("#filename").WaitFor(); err != nil {
		log.Fatalf("could not wait for firmware upgrade page: %v", err)
	}

	// Read current versions
	fwVersion, _ := mainFrame.Locator("#up_sver").InnerText()
	hwVersion, _ := mainFrame.Locator("#up_hver").InnerText()
	fmt.Printf("Firmware Version: %s\n", fwVersion)
	fmt.Printf("Hardware Version: %s\n", hwVersion)

	// Upload firmware file
	fmt.Printf("Uploading firmware: %s\n", firmwarePath)
	_ = mainFrame.Locator("#filename").SetInputFiles(firmwarePath)
	time.Sleep(1 * time.Second)

	// Handle dialog (upgrade confirmation)
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// Click Upgrade button
	fmt.Println("Clicking Upgrade button...")
	_ = mainFrame.Locator("#t_upgrade").Click()

	// Wait for router to reboot (this takes a while)
	fmt.Println("Waiting for router to reboot (60s)...")
	time.Sleep(60 * time.Second)

	fmt.Println("PoC Finished.")
}
