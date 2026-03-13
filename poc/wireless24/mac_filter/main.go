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
		Headless: playwright.Bool(false), // Set to true for production or headless environments
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	// === Configuration values ===
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"
	enableFilter := true
	rule := "allow" // "allow" or "deny"

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 2. Login
	fmt.Println("Attempting login...")
	if err := page.Locator("#userName").WaitFor(); err != nil {
		log.Fatalf("could not wait for username input: %v", err)
	}
	if err := page.Locator("#userName").Fill(username); err != nil {
		log.Fatalf("could not fill username: %v", err)
	}
	if err := page.Locator("#pcPassword").Fill(password); err != nil {
		log.Fatalf("could not fill password: %v", err)
	}
	if err := page.Locator("#loginBtn").Click(); err != nil {
		log.Fatalf("could not click login: %v", err)
	}

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// 3. Find bottomLeftFrame
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

	// 4. Click "Wireless 2.4GHz" menu
	fmt.Println("Clicking Wireless 2.4GHz menu...")
	wireless24Loc := leftFrame.Locator("a:has-text('ワイヤレス 2.4GHz'), a:has-text('Wireless 2.4GHz')").First()
	if err := wireless24Loc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Wireless 2.4GHz menu: %v", err)
	}
	if err := wireless24Loc.Click(); err != nil {
		log.Fatalf("could not click Wireless 2.4GHz menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// 5. Click "Wireless MAC Filtering" submenu
	fmt.Println("Clicking Wireless MAC Filtering submenu...")
	macFilterLoc := leftFrame.Locator("a:has-text('ワイヤレス MAC フィルタリング'), a:has-text('Wireless MAC Filtering')").First()
	if err := macFilterLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Wireless MAC Filtering link: %v", err)
	}
	if err := macFilterLoc.Click(); err != nil {
		log.Fatalf("could not click Wireless MAC Filtering submenu: %v", err)
	}

	// 6. Find mainFrame
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

	// 7. Enable/Disable MAC Filtering
	if enableFilter {
		fmt.Println("Enabling MAC Filtering...")
		btn := mainFrame.Locator("input#acl_en").First()
		if err := btn.WaitFor(); err != nil {
			log.Fatalf("could not wait for enable button: %v", err)
		}
		if err := btn.Click(); err != nil {
			log.Fatalf("could not click enable button: %v", err)
		}
	} else {
		fmt.Println("Disabling MAC Filtering...")
		btn := mainFrame.Locator("input#acl_dis").First()
		if err := btn.WaitFor(); err != nil {
			log.Fatalf("could not wait for disable button: %v", err)
		}
		if err := btn.Click(); err != nil {
			log.Fatalf("could not click disable button: %v", err)
		}
	}

	// 8. Set Filtering Rule
	if rule == "allow" {
		fmt.Println("Setting filtering rule to Allow...")
		radio := mainFrame.Locator("input#allow").First()
		if err := radio.WaitFor(); err != nil {
			log.Fatalf("could not wait for allow radio: %v", err)
		}
		if err := radio.Click(); err != nil {
			log.Fatalf("could not click allow radio: %v", err)
		}
	} else {
		fmt.Println("Setting filtering rule to Deny...")
		radio := mainFrame.Locator("input#deny").First()
		if err := radio.WaitFor(); err != nil {
			log.Fatalf("could not wait for deny radio: %v", err)
		}
		if err := radio.Click(); err != nil {
			log.Fatalf("could not click deny radio: %v", err)
		}
	}

	// 9. Add New Entry button
	fmt.Println("Clicking 'Add New' button...")
	addBtn := mainFrame.Locator("input.T_addnew").First()
	if err := addBtn.WaitFor(); err != nil {
		log.Printf("could not wait for Add New button: %v", err)
	} else {
		// Note: This will usually navigate to a new page or open a dialog/frame
		if err := addBtn.Click(); err != nil {
			log.Printf("could not click Add New button: %v", err)
		}
	}

	fmt.Println("Waiting for router to apply settings (3s)...")
	time.Sleep(3 * time.Second)

	fmt.Println("\nPoC Finished successfully.")
}
