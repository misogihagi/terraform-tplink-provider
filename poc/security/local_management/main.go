package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings to apply to the Local Management (ローカル管理) page.
type Config struct {
	// AllowAll determines the management rule.
	//   true  -> "すべて" (All): every PC on the LAN can access the router's Web-based utility.
	//   false -> "のみ" (Only): only the PC with MACAddress can browse and manage.
	AllowAll bool

	// MACAddress is the MAC address of the only PC allowed to manage when AllowAll is false.
	// Leave empty to fill it with the current PC's MAC via the "セット" (Set) button.
	MACAddress string
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		AllowAll:   false,
		MACAddress: "AA:BB:CC:DD:EE:FF",
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

	// 4. Find bottomLeftFrame and navigate to Local Management
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

	fmt.Println("Clicking Local Management submenu...")
	localMgmtLoc := leftFrame.Locator("a:has-text('ローカル管理'), a:has-text('Local Management')").First()
	if err := localMgmtLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for local management link: %v", err)
	}
	if err := localMgmtLoc.Click(); err != nil {
		log.Fatalf("could not click local management submenu: %v", err)
	}

	// 5. Find mainFrame and interact with the management rule radios
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

	fmt.Println("Waiting for management rule controls...")
	actAll := mainFrame.Locator("input#act_all")
	if err := actAll.WaitFor(); err != nil {
		log.Fatalf("could not wait for management rule radio: %v", err)
	}

	// 6. Select the management rule
	fmt.Printf("Setting management rule allow_all=%v...\n", cfg.AllowAll)
	if cfg.AllowAll {
		if err := actAll.Check(); err != nil {
			log.Fatalf("could not check 'すべて' radio: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#act_cet").Check(); err != nil {
			log.Fatalf("could not check 'のみ' radio: %v", err)
		}
	}

	// Wait briefly for the UI to update (selecting 'のみ' enables mac1/setMac)
	time.Sleep(500 * time.Millisecond)

	if !cfg.AllowAll {
		// 7. Read the current PC's MAC address
		curMac, err := mainFrame.Locator("input#curMac").InputValue()
		if err != nil {
			log.Printf("could not read current MAC: %v", err)
		} else {
			fmt.Printf("Current PC MAC address: %s\n", curMac)
		}

		// 8. Set the allowed MAC address
		if cfg.MACAddress == "" {
			fmt.Println("No MAC specified, using the 'セット' button to copy the current PC's MAC...")
			setMac := mainFrame.Locator("input#setMac")
			if err := setMac.WaitFor(); err != nil {
				log.Fatalf("could not wait for setMac button: %v", err)
			}
			if err := setMac.Click(); err != nil {
				log.Fatalf("could not click setMac button: %v", err)
			}
			// Wait briefly for the value to be copied into #mac1
			time.Sleep(500 * time.Millisecond)
		} else {
			fmt.Printf("Filling allowed MAC address %s...\n", cfg.MACAddress)
			if err := mainFrame.Locator("input#mac1").Fill(cfg.MACAddress); err != nil {
				log.Fatalf("could not fill allowed MAC: %v", err)
			}
		}

		macValue, err := mainFrame.Locator("input#mac1").InputValue()
		if err != nil {
			log.Printf("could not read MAC input value: %v", err)
		} else {
			fmt.Printf("Allowed MAC address set to: %s\n", macValue)
		}
	}

	// 9. Save the settings
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
