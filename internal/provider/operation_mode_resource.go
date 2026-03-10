package main

import (
	"fmt"
	"log"

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

	context, err := browser.NewContext()
	if err != nil {
		log.Fatalf("could not create context: %v", err)
	}

	page, err := context.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	fmt.Println("Navigating to 192.168.1.1...")
	if _, err = page.Goto("http://192.168.1.1"); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	fmt.Println("Typing credentials...")
	// We wait for the #userName element to appear to ensure the page has loaded
	if err = page.Locator("#userName").WaitFor(); err != nil {
		log.Printf("could not wait for username input: %v", err)
	}
	if err = page.Locator("#userName").Fill("admin"); err != nil {
		log.Printf("could not fill username: %v", err)
	}
	if err = page.Locator("#pcPassword").Fill("admin"); err != nil {
		log.Printf("could not fill password: %v", err)
	}

	fmt.Println("Clicking login...")
	if err = page.Locator("#loginBtn").Click(); err != nil {
		log.Printf("could not click login: %v", err)
	}

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	fmt.Println("Waiting for bottomLeftFrame...")
	// The frames might not be ready immediately after login.
	// We need to wait for the bottomLeftFrame explicitly.
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
		page.WaitForTimeout(500)
	}

	if leftFrame == nil {
		log.Fatalf("could not find bottomLeftFrame")
	}

	fmt.Println("Clicking Operation Mode...")
	// Click the element with ID a1 or text "動作するモード" or "Operation Mode"
	// From the trace, the ID is typically 'a1' or 'sysmod', or we can find by text.
	loc := leftFrame.Locator("a:has-text('動作するモード'), a:has-text('Operation Mode'), #a1").First()
	if err := loc.WaitFor(); err != nil {
		log.Printf("could not wait for operation mode link: %v", err)
	}
	if err := loc.Click(); err != nil {
		log.Fatalf("could not click operation mode menu: %v", err)
	}

	fmt.Println("Waiting for mainFrame to load...")
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
		page.WaitForTimeout(500)
	}

	if mainFrame == nil {
		log.Fatalf("could not find mainFrame")
	}

	fmt.Println("Selecting Access Point mode...")
	// Wait for the AP radio button. The IDs found were Router, Hotspot, AP, Repeater, Client, MSSID
	apRadio := mainFrame.Locator("input#AP")
	if err := apRadio.WaitFor(); err != nil {
		log.Printf("could not wait for AP radio button: %v", err)
	}
	if err := apRadio.Click(); err != nil {
		log.Fatalf("could not click AP mode: %v", err)
	}

	fmt.Println("Clicking Save...")
	saveBtn := mainFrame.Locator("input#b_save, input.buttonBig[value*='保存'], input.buttonBig[value*='Save']").First()
	if err := saveBtn.Click(); err != nil {
		log.Fatalf("could not click save: %v", err)
	}

	fmt.Println("Save clicked. Waiting to observe...")
	page.WaitForTimeout(3000)

	fmt.Println("Done")
}
