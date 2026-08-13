package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings to apply to the Remote Management (リモート管理) page.
type Config struct {
	// HTTPPort is the Web management port (e.g. "8080", 1-65535).
	HTTPPort string

	// RemoteHost is the remote management IP address.
	// Enter "255.255.255.255" to allow all remote PCs.
	RemoteHost string
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		HTTPPort:   "8080",
		RemoteHost: "255.255.255.255",
	}

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

	// 4. Find bottomLeftFrame and navigate to Remote Management
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

	fmt.Println("Clicking Remote Management submenu...")
	remoteMgmtLoc := leftFrame.Locator("a:has-text('リモート管理'), a:has-text('Remote Management')").First()
	if err := remoteMgmtLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for remote management link: %v", err)
	}
	if err := remoteMgmtLoc.Click(); err != nil {
		log.Fatalf("could not click remote management submenu: %v", err)
	}

	// 5. Find mainFrame and interact with the remote management inputs
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

	fmt.Println("Waiting for remote management controls...")
	httpPort := mainFrame.Locator("input#r_http_port")
	if err := httpPort.WaitFor(); err != nil {
		log.Fatalf("could not wait for web management port input: %v", err)
	}

	// 6. Set the Web management port
	fmt.Printf("Setting Web management port to %s...\n", cfg.HTTPPort)
	if err := httpPort.Fill(cfg.HTTPPort); err != nil {
		log.Fatalf("could not fill web management port: %v", err)
	}

	// 7. Set the remote management IP address
	fmt.Printf("Setting remote management IP address to %s...\n", cfg.RemoteHost)
	remoteHost := mainFrame.Locator("input#r_host")
	if err := remoteHost.Fill(cfg.RemoteHost); err != nil {
		log.Fatalf("could not fill remote management IP address: %v", err)
	}

	// 8. Save the settings
	fmt.Println("Clicking Save button...")
	saveBtn := mainFrame.Locator("input.button.L.T.T_save, input[value='保存'], input[value='Save']").First()

	// Capture any confirmation dialogs
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
