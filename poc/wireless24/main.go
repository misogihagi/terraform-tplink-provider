package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
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

	// 2. Browser Context with basic auth
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		HttpCredentials: &playwright.HttpCredentials{
			Username: "admin",
			Password: "admin",
		},
	})
	if err != nil {
		log.Fatalf("could not create context: %v", err)
	}

	page, err := context.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	endpoint := "http://192.168.1.1"
	newSSID := "TP-Link_2.4GHz_PoC"
	newPassword := "password123"

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 3. Handle login
	fmt.Println("Waiting for login elements...")
	_ = page.Locator("#userName").Fill("admin")
	_ = page.Locator("#pcPassword").Fill("admin")
	_ = page.Locator("#loginBtn").Click()

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// 4. Extract frames and navigate
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

	fmt.Println("Clicking Wireless 2.4GHz menu...")
	// Wireless 2.4GHz menu might have different text or ID
	wireless24Loc := leftFrame.Locator("a:has-text('ワイヤレス 2.4GHz'), a:has-text('Wireless 2.4GHz'), #a7").First()
	if err := wireless24Loc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Wireless 2.4GHz link: %v", err)
	}
	if err := wireless24Loc.Click(); err != nil {
		log.Fatalf("could not click Wireless 2.4GHz menu: %v", err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Wireless Settings submenu...")
	settingsLoc := leftFrame.Locator("a:has-text('ワイヤレス設定'), a:has-text('Wireless Settings'), #a8").First()
	if err := settingsLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for wireless settings link: %v", err)
	}
	if err := settingsLoc.Click(); err != nil {
		log.Fatalf("could not click wireless settings submenu: %v", err)
	}

	// 5. Update Wireless Settings in mainFrame
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

	fmt.Printf("Filling new SSID: %s\n", newSSID)
	ssidInput := mainFrame.Locator("input#ssidName, input[name='ssidName']").First()
	if err := ssidInput.WaitFor(); err != nil {
		log.Fatalf("could not wait for SSID input: %v", err)
	}
	if err := ssidInput.Fill(newSSID); err != nil {
		log.Fatalf("could not fill SSID: %v", err)
	}

	fmt.Printf("Filling new Password: %s\n", newPassword)
	// Some routers might have multiple password fields or different IDs
	pwdInput := mainFrame.Locator("input#pskSecret, input[name='pskSecret'], input#password").First()
	if err := pwdInput.WaitFor(); err != nil {
		log.Fatalf("could not wait for Password input: %v", err)
	}
	if err := pwdInput.Fill(newPassword); err != nil {
		log.Fatalf("could not fill Password: %v", err)
	}

	// 6. Handle Save
	fmt.Println("Clicking Save button...")
	saveBtn := mainFrame.Locator("input#b_save, input.buttonBig[value*='保存'], input.buttonBig[value*='Save']").First()

	// Capture dialog for confirmation
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		dialog.Accept()
	})

	if err := saveBtn.Click(); err != nil {
		log.Fatalf("could not click save: %v", err)
	}

	fmt.Println("Waiting for router to process request (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished")
}
