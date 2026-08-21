package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings for Access Control Target (アクセス制御 > ターゲット) entries.
type Config struct {
	// Targets is the list of target entries to manage.
	Targets []Target
}

// Target represents a single target entry.
type Target struct {
	// Description is a human-readable name for the target (max 15 chars).
	Description string

	// Mode: "ip", "mac", or "url"
	Mode string

	// IP mode fields
	IPStart   string // Start IP address
	IPEnd     string // End IP address
	PortStart string // Start port
	PortEnd   string // End port

	// MAC mode field
	MACAddr string // MAC address

	// URL mode fields
	URLs []string // List of URLs
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		Targets: []Target{
			{
				Description: "WebBlock",
				Mode:        "url",
				URLs:        []string{"facebook.com", "twitter.com"},
			},
			{
				Description: "LocalNet",
				Mode:        "ip",
				IPStart:     "192.168.1.0",
				IPEnd:       "192.168.1.255",
				PortStart:   "80",
				PortEnd:     "443",
			},
			{
				Description: "PrinterMAC",
				Mode:        "mac",
				MACAddr:     "AA:BB:CC:DD:EE:FF",
			},
		},
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
		Headless: playwright.Bool(false),
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

	// 4. Find bottomLeftFrame and navigate to Access Control > Target
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

	fmt.Println("Clicking Access Control menu...")
	aclLoc := leftFrame.Locator("a:has-text('アクセス制御'), a:has-text('Access Control')").First()
	if err := aclLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Access Control link: %v", err)
	}
	if err := aclLoc.Click(); err != nil {
		log.Fatalf("could not click Access Control menu: %v", err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Target submenu...")
	targetLoc := leftFrame.Locator("a:has-text('ターゲット'), a:has-text('Target')").First()
	if err := targetLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Target link: %v", err)
	}
	if err := targetLoc.Click(); err != nil {
		log.Fatalf("could not click Target submenu: %v", err)
	}

	// 5. Find mainFrame
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

	fmt.Println("Waiting for target list page...")
	if err := mainFrame.Locator("#wantbl, table#wantbl").WaitFor(); err != nil {
		log.Printf("wait for target table: %v", err)
	}

	// Handle dialog confirmations
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// 6. Add target entries
	for i, target := range cfg.Targets {
		fmt.Printf("Adding target %d: %s...\n", i+1, target.Description)

		// Click "Add New" button
		addBtn := mainFrame.Locator("input.T_addnew, input[value='新規追加'], input[value='Add New']").First()
		if err := addBtn.Click(); err != nil {
			log.Fatalf("could not click Add New button: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Fill in Description
		descInput := mainFrame.Locator("input#entryName, input[name='entryName']").First()
		if err := descInput.WaitFor(); err != nil {
			log.Fatalf("could not wait for description input: %v", err)
		}
		if err := descInput.Fill(target.Description); err != nil {
			log.Fatalf("could not fill description: %v", err)
		}

		// Set Mode
		modeSelect := mainFrame.Locator("select#mode").First()
		if err := modeSelect.WaitFor(); err != nil {
			log.Fatalf("could not wait for mode select: %v", err)
		}

		switch target.Mode {
		case "mac":
			// Select MAC Address mode (value="1")
			fmt.Println("Selecting MAC Address mode...")
			if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"1"},
			}); err != nil {
				log.Fatalf("could not select MAC mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			// Fill MAC address
			macInput := mainFrame.Locator("input#macAddr, input[name='macAddr']").First()
			if err := macInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for MAC input: %v", err)
			}
			if err := macInput.Fill(target.MACAddr); err != nil {
				log.Fatalf("could not fill MAC address: %v", err)
			}

		case "url":
			// Select URL Address mode (value="2")
			fmt.Println("Selecting URL Address mode...")
			if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"2"},
			}); err != nil {
				log.Fatalf("could not select URL mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			// Add URLs
			for _, url := range target.URLs {
				if url == "" {
					continue
				}
				fmt.Printf("  Adding URL: %s\n", url)

				urlInput := mainFrame.Locator("input#urlInfo, input[name='urlInfo']").First()
				if err := urlInput.WaitFor(); err != nil {
					log.Fatalf("could not wait for URL input: %v", err)
				}
				if err := urlInput.Fill(url); err != nil {
					log.Fatalf("could not fill URL: %v", err)
				}

				// Click Add button for URL
				addUrlBtn := mainFrame.Locator("input[onclick='doAddUrl();'], input.T_add, input[value='追加'], input[value='Add']").First()
				if err := addUrlBtn.Click(); err != nil {
					log.Fatalf("could not click Add URL button: %v", err)
				}
				time.Sleep(500 * time.Millisecond)
			}

		default: // "ip"
			// Select IP Address mode (value="0")
			fmt.Println("Selecting IP Address mode...")
			if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"0"},
			}); err != nil {
				log.Fatalf("could not select IP mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			// Fill IP Start
			ipStartInput := mainFrame.Locator("input#ipStart, input[name='ipStart']").First()
			if err := ipStartInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for IP start input: %v", err)
			}
			if err := ipStartInput.Fill(target.IPStart); err != nil {
				log.Fatalf("could not fill IP start: %v", err)
			}

			// Fill IP End
			ipEndInput := mainFrame.Locator("input#ipEnd, input[name='ipEnd']").First()
			if err := ipEndInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for IP end input: %v", err)
			}
			if err := ipEndInput.Fill(target.IPEnd); err != nil {
				log.Fatalf("could not fill IP end: %v", err)
			}

			// Fill Port Start
			portStartInput := mainFrame.Locator("input#portStart, input[name='portStart']").First()
			if err := portStartInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for port start input: %v", err)
			}
			if err := portStartInput.Fill(target.PortStart); err != nil {
				log.Fatalf("could not fill port start: %v", err)
			}

			// Fill Port End
			portEndInput := mainFrame.Locator("input#portEnd, input[name='portEnd']").First()
			if err := portEndInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for port end input: %v", err)
			}
			if err := portEndInput.Fill(target.PortEnd); err != nil {
				log.Fatalf("could not fill port end: %v", err)
			}
		}

		// Click Save
		fmt.Printf("Saving target %d...\n", i+1)
		saveBtn := mainFrame.Locator("input#saveBtn, input.T_save, input[value='保存'], input[value='Save']").First()
		if err := saveBtn.Click(); err != nil {
			log.Fatalf("could not click save button: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("PoC Finished")
}
