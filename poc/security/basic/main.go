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

	// 2. Browser Context
	context, err := browser.NewContext()
	if err != nil {
		log.Fatalf("could not create context: %v", err)
	}

	page, err := context.NewPage()
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

	fmt.Println("Clicking Security menu...")
	securityLoc := leftFrame.Locator("a:has-text('セキュリティ'), a:has-text('Security')").First()
	if err := securityLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for security link: %v", err)
	}
	if err := securityLoc.Click(); err != nil {
		log.Fatalf("could not click security menu: %v", err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Basic Security submenu...")
	basicSecurityLoc := leftFrame.Locator("a:has-text('基本セキュリティ'), a:has-text('Basic Security')").First()
	if err := basicSecurityLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for basic security link: %v", err)
	}
	if err := basicSecurityLoc.Click(); err != nil {
		log.Fatalf("could not click basic security submenu: %v", err)
	}

	// 5. Interact with elements in mainFrame
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

	// SPI Firewall
	fmt.Println("Configuring SPI Firewall...")
	enableSpi := mainFrame.Locator("input#enable_spi")
	if err := enableSpi.SetChecked(true); err != nil {
		log.Printf("could not set SPI firewall: %v", err)
	}

	// VPN Passthrough
	fmt.Println("Configuring VPN Passthrough...")
	_ = mainFrame.Locator("input#pptpEnable").Check()
	_ = mainFrame.Locator("input#l2tpEnable").Check()
	_ = mainFrame.Locator("input#ipSecEnable").Check()

	// ALG
	fmt.Println("Configuring ALG...")
	_ = mainFrame.Locator("input#ftpEnable").Check()
	_ = mainFrame.Locator("input#tftpEnable").Check()
	_ = mainFrame.Locator("input#h323Enable").Check()
	_ = mainFrame.Locator("input#sipEnable").Check()
	_ = mainFrame.Locator("input#rtspEnable").Check()

	// 6. Handle Save
	fmt.Println("Clicking Save button...")
	saveBtn := mainFrame.Locator("input.button.L.T.T_save, input[value='保存'], input[value='Save']").First()

	// Capture dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		dialog.Accept()
	})

	if err := saveBtn.Click(); err != nil {
		log.Fatalf("could not click save: %v", err)
	}

	fmt.Println("Waiting for router to process request (3s)...")
	time.Sleep(3 * time.Second)

	fmt.Println("PoC Finished")
}
