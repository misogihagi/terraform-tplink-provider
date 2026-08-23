package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings to apply to the Static Route List (静的経路リスト) page.
type Config struct {
	// Routes is the list of static route entries to add.
	Routes []StaticRoute
}

// StaticRoute represents a single static route entry.
type StaticRoute struct {
	// Destination is the destination IP address (宛先 IP アドレス).
	Destination string

	// Netmask is the subnet mask (サブネット マスク).
	Netmask string

	// Gateway is the gateway IP address (ゲートウェイ).
	Gateway string

	// Interface is the WAN interface (インターフェイス):
	//   "Internet" -> インターネット接続
	//   "LAN"      -> LAN & WLAN
	// Use "" for the empty default option.
	Interface string

	// Enabled determines whether the entry is enabled (ステータス).
	Enabled bool
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		Routes: []StaticRoute{
			{
				Destination: "10.0.0.0",
				Netmask:     "255.0.0.0",
				Gateway:     "192.168.1.254",
				Interface:   "Internet", // インターネット接続
				Enabled:     true,
			},
			{
				Destination: "172.16.0.0",
				Netmask:     "255.240.0.0",
				Gateway:     "192.168.1.253",
				Interface:   "LAN", // LAN & WLAN
				Enabled:     false,
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

	// 4. Find bottomLeftFrame and navigate to Advanced Routing > Static Routing List
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

	fmt.Println("Clicking Advanced Routing menu...")
	routingLoc := leftFrame.Locator("a:has-text('高度な経路'), a:has-text('Advanced Routing')").First()
	if err := routingLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Advanced Routing link: %v", err)
	}
	if err := routingLoc.Click(); err != nil {
		log.Fatalf("could not click Advanced Routing menu: %v", err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Static Routing submenu...")
	staticLoc := leftFrame.Locator("a:has-text('静的経路'), a:has-text('Static Routing')").First()
	if err := staticLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Static Routing link: %v", err)
	}
	if err := staticLoc.Click(); err != nil {
		log.Fatalf("could not click Static Routing submenu: %v", err)
	}

	// 5. Find mainFrame and interact with the controls
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

	fmt.Println("Waiting for static route table...")
	if err := mainFrame.Locator("#staticRtetbl, table#staticRtetbl").WaitFor(); err != nil {
		log.Printf("wait for static route table: %v", err)
	}

	// Capture any confirmation dialogs
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		dialog.Accept()
	})

	// 6. Add static route entries
	for i, route := range cfg.Routes {
		fmt.Printf("Adding route %d: %s...\n", i+1, route.Destination)

		// Click "Add New" button (doAdd)
		addBtn := mainFrame.Locator("input.T_addnew, input[value='新規追加'], input[value='Add New']").First()
		if err := addBtn.Click(); err != nil {
			log.Fatalf("could not click Add New button: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Fill in Destination IP address
		destInput := mainFrame.Locator("input#desAddr").First()
		if err := destInput.WaitFor(); err != nil {
			log.Fatalf("could not wait for destination input: %v", err)
		}
		if err := destInput.Fill(route.Destination); err != nil {
			log.Fatalf("could not fill destination: %v", err)
		}

		// Fill in Subnet Mask
		maskInput := mainFrame.Locator("input#mask").First()
		if err := maskInput.WaitFor(); err != nil {
			log.Fatalf("could not wait for netmask input: %v", err)
		}
		if err := maskInput.Fill(route.Netmask); err != nil {
			log.Fatalf("could not fill netmask: %v", err)
		}

		// Fill in Gateway
		gwInput := mainFrame.Locator("input#defGw").First()
		if err := gwInput.WaitFor(); err != nil {
			log.Fatalf("could not wait for gateway input: %v", err)
		}
		if err := gwInput.Fill(route.Gateway); err != nil {
			log.Fatalf("could not fill gateway: %v", err)
		}

		// Select Interface
		if route.Interface != "" {
			ifaceSelect := mainFrame.Locator("select#wanInf").First()
			if err := ifaceSelect.WaitFor(); err != nil {
				log.Fatalf("could not wait for interface select: %v", err)
			}
			if _, err := ifaceSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{route.Interface},
			}); err != nil {
				log.Fatalf("could not select interface: %v", err)
			}
		}

		// Set Status (enabled/disabled)
		stateValue := "0"
		if route.Enabled {
			stateValue = "1"
		}
		stateSelect := mainFrame.Locator("select#state").First()
		if err := stateSelect.WaitFor(); err != nil {
			log.Fatalf("could not wait for state select: %v", err)
		}
		if _, err := stateSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{stateValue},
		}); err != nil {
			log.Fatalf("could not select state: %v", err)
		}

		// Save the entry
		fmt.Printf("Saving route %d...\n", i+1)
		saveBtn := mainFrame.Locator("input#saveBtn, input.T_save, input[value='保存'], input[value='Save']").First()
		if err := saveBtn.Click(); err != nil {
			log.Fatalf("could not click save button: %v", err)
		}
		time.Sleep(2 * time.Second)

		// Go back to the route list if still on the edit page
		backBtn := mainFrame.Locator("input.T_back").First()
		if err := backBtn.Click(); err != nil {
			log.Printf("could not click back button (maybe already back): %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("PoC Finished")
}
