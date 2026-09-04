package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// BindingEntry represents a single IP & MAC binding entry
// (IP-MAC バインディング エントリ).
type BindingEntry struct {
	// MACAddress is the MAC address of the device (MAC アドレス).
	MACAddress string

	// IPAddress is the IP address to bind to the MAC address (IP アドレス).
	IPAddress string

	// Enabled determines whether the entry is bound (バインド).
	Enabled bool
}

func main() {
	// --- Target configuration ---
	cfg := struct {
		// ArpEnabled enables the whole ARP binding feature (ARP バインディング).
		ArpEnabled bool

		Entries []BindingEntry
	}{
		ArpEnabled: true,
		Entries: []BindingEntry{
			{
				MACAddress: "AA-BB-CC-DD-EE-F1",
				IPAddress:  "192.168.1.100",
				Enabled:    true,
			},
			{
				MACAddress: "AA-BB-CC-DD-EE-F2",
				IPAddress:  "192.168.1.101",
				Enabled:    false,
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

	// Capture any confirmation dialogs
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		dialog.Accept()
	})

	// 4. Find bottomLeftFrame and navigate to IP & MAC Binding > Binding Settings
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

	fmt.Println("Clicking IP & MAC Binding menu...")
	bindLoc := leftFrame.Locator("a:has-text('MAC バインディング'), a:has-text('MAC Binding')").First()
	if err := bindLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for IP & MAC Binding menu: %v", err)
	}
	if err := bindLoc.Click(); err != nil {
		log.Fatalf("could not click IP & MAC Binding menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Binding Settings submenu...")
	settingsLoc := leftFrame.Locator("a:has-text('バインディング 設定'), a:has-text('バインディング設定'), a:has-text('Binding Settings'), a:has-text('Binding')").First()
	if err := settingsLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Binding Settings link: %v", err)
	}
	if err := settingsLoc.Click(); err != nil {
		log.Fatalf("could not click Binding Settings submenu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// 5. Find mainFrame and configure ARP binding
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

	fmt.Println("Waiting for binding settings page...")
	if err := mainFrame.Locator("input#arpBind_en").WaitFor(); err != nil {
		log.Printf("wait for arpBind_en radio: %v", err)
	}

	// Enable/disable ARP binding and save
	if cfg.ArpEnabled {
		fmt.Println("Enabling ARP binding...")
		if err := mainFrame.Locator("input#arpBind_en").Check(); err != nil {
			log.Fatalf("could not check arpBind_en: %v", err)
		}
	} else {
		fmt.Println("Disabling ARP binding...")
		if err := mainFrame.Locator("input#arpBind_dis").Check(); err != nil {
			log.Fatalf("could not check arpBind_dis: %v", err)
		}
	}

	saveArpBtn := mainFrame.Locator("input#saveArpEn").First()
	if err := saveArpBtn.Click(); err != nil {
		log.Fatalf("could not click saveArpEn button: %v", err)
	}
	time.Sleep(2 * time.Second)

	// 6. Add binding entries
	for i, entry := range cfg.Entries {
		fmt.Printf("Adding binding entry %d: %s -> %s...\n", i+1, entry.MACAddress, entry.IPAddress)

		addBtn := mainFrame.Locator("input.T_addnew, input[value='新規追加'], input[value='Add New']").First()
		if err := addBtn.Click(); err != nil {
			log.Fatalf("could not click Add New button: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Fill in MAC address
		macInput := mainFrame.Locator("input#macAddr").First()
		if err := macInput.WaitFor(); err != nil {
			log.Fatalf("could not wait for macAddr input: %v", err)
		}
		if err := macInput.Fill(entry.MACAddress); err != nil {
			log.Fatalf("could not fill macAddr: %v", err)
		}

		// Fill in IP address
		ipInput := mainFrame.Locator("input#ipAddr").First()
		if err := ipInput.Fill(entry.IPAddress); err != nil {
			log.Fatalf("could not fill ipAddr: %v", err)
		}

		// Set Bind status
		if entry.Enabled {
			if err := mainFrame.Locator("input#arpBind").Check(); err != nil {
				log.Printf("could not check arpBind: %v", err)
			}
		} else {
			_ = mainFrame.Locator("input#arpBind").Uncheck()
		}

		// Save the entry
		fmt.Printf("Saving entry %d...\n", i+1)
		saveBtn := mainFrame.Locator("input#saveBtn, input.T_save").First()
		if err := saveBtn.Click(); err != nil {
			log.Fatalf("could not click save button: %v", err)
		}
		time.Sleep(2 * time.Second)

		// Go back to the entry list
		backBtn := mainFrame.Locator("input.T_back").First()
		if err := backBtn.Click(); err != nil {
			log.Printf("could not click back button (maybe already back): %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	// 7. Read the resulting binding list
	fmt.Println("Reading IP & MAC binding list...")
	rows, _ := mainFrame.Locator("table#arptbl tr").All()
	if len(rows) == 0 {
		fmt.Println("Binding list is empty.")
	} else {
		fmt.Println("Checked\tMAC Address\tIP Address\tBound")
		for _, row := range rows {
			cols, _ := row.Locator("td").AllInnerTexts()
			if len(cols) > 0 {
				fmt.Println(strings.Join(cols, "\t"))
			}
		}
	}

	fmt.Println("PoC Finished")
}
