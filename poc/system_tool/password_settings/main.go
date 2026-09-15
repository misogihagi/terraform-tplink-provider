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

	newUsername := "admin2"
	newPassword := "admin2"

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

	// Click Password submenu
	fmt.Println("Clicking Password submenu...")
	passwordMenuLoc := leftFrame.Locator("a:has-text('パスワード'), a:has-text('Password')").First()
	if err := passwordMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Password submenu: %v", err)
	}
	_ = passwordMenuLoc.Click()
	time.Sleep(1 * time.Second)

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

	// Wait for the password page
	if err := mainFrame.Locator("#curName").WaitFor(); err != nil {
		log.Fatalf("could not wait for password page to load: %v", err)
	}

	// Read current username (may be pre-filled by the router)
	curNameVal, err := mainFrame.Locator("#curName").InputValue()
	if err == nil {
		fmt.Printf("Current username value: %s\n", curNameVal)
	}

	// Fill in old credentials
	fmt.Println("Filling old credentials...")
	_ = mainFrame.Locator("#curName").Fill(username)
	_ = mainFrame.Locator("#curPwd").Fill(password)

	// Fill in new credentials
	fmt.Printf("Setting new username: %s\n", newUsername)
	_ = mainFrame.Locator("#newName").Fill(newUsername)
	_ = mainFrame.Locator("#newPwd").Fill(newPassword)
	_ = mainFrame.Locator("#cfmPwd").Fill(newPassword)

	// Handle confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// Click Save button
	fmt.Println("Clicking Save button...")
	_ = mainFrame.Locator("input.button:has-text('保存'), input.button:has-text('Save')").Click()

	// Wait for router to apply
	fmt.Println("Waiting for router to apply (5s)...")
	time.Sleep(5 * time.Second)

	// Re-login with new credentials to verify
	fmt.Println("Verifying new credentials by re-login...")
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	if err := page.Locator("#userName").WaitFor(); err != nil {
		log.Fatalf("could not wait for login page: %v", err)
	}
	_ = page.Locator("#userName").Fill(newUsername)
	_ = page.Locator("#pcPassword").Fill(newPassword)
	_ = page.Locator("#loginBtn").Click()

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	fmt.Println("PoC Finished.")
}
