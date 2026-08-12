package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings to apply to the Advanced Security page.
type Config struct {
	// DoSProtectionEnabled enables or disables the DoS protection feature.
	DoSProtectionEnabled bool

	// ICMPFloodFilter enables ICMP-Flood attack filtering.
	ICMPFloodFilter bool
	// ICMPThreshold is the packet threshold for ICMP-Flood (5-3600 packets/sec).
	ICMPThreshold string

	// UDPFloodFilter enables UDP-Flood attack filtering.
	UDPFloodFilter bool
	// UDPThreshold is the packet threshold for UDP-Flood (5-3600 packets/sec).
	UDPThreshold string

	// SYNFloodFilter enables TCP-SYN-Flood attack filtering.
	SYNFloodFilter bool
	// SYNThreshold is the packet threshold for TCP-SYN-Flood (5-3600 packets/sec).
	SYNThreshold string

	// WANPingFilter disallows Ping packets from the WAN port.
	WANPingFilter bool
	// LANPingFilter disallows Ping packets from the LAN port.
	LANPingFilter bool
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		DoSProtectionEnabled: true,

		ICMPFloodFilter: true,
		ICMPThreshold:   "50",

		UDPFloodFilter: true,
		UDPThreshold:   "500",

		SYNFloodFilter: true,
		SYNThreshold:   "30",

		WANPingFilter: true,
		LANPingFilter: false,
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

	// 4. Find bottomLeftFrame and navigate to Advanced Security
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

	fmt.Println("Clicking Advanced Security submenu...")
	advSecLoc := leftFrame.Locator("a:has-text('高度セキュリティ'), a:has-text('Advanced Security')").First()
	if err := advSecLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for advanced security link: %v", err)
	}
	if err := advSecLoc.Click(); err != nil {
		log.Fatalf("could not click advanced security submenu: %v", err)
	}

	// 5. Find mainFrame and interact with DoS protection elements
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

	// Wait for the DoS Protection radio buttons to be ready
	fmt.Println("Waiting for DoS Protection controls...")
	dosEnable := mainFrame.Locator("input#ddos_en")
	if err := dosEnable.WaitFor(); err != nil {
		log.Fatalf("could not wait for DoS enable radio: %v", err)
	}

	// 6. Configure DoS Protection (enable/disable)
	fmt.Printf("Setting DoS Protection enabled=%v...\n", cfg.DoSProtectionEnabled)
	if cfg.DoSProtectionEnabled {
		if err := mainFrame.Locator("input#ddos_en").Click(); err != nil {
			log.Fatalf("could not enable DoS protection: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#ddos_dis").Click(); err != nil {
			log.Fatalf("could not disable DoS protection: %v", err)
		}
	}

	// Wait briefly for the UI to update (enabling DoS unlocks the filter checkboxes)
	time.Sleep(500 * time.Millisecond)

	if cfg.DoSProtectionEnabled {
		// 7. Configure ICMP-Flood filter
		fmt.Printf("Setting ICMP-Flood filter=%v, threshold=%s...\n", cfg.ICMPFloodFilter, cfg.ICMPThreshold)
		if err := mainFrame.Locator("input#icmpFilter").SetChecked(cfg.ICMPFloodFilter); err != nil {
			log.Printf("could not set ICMP filter: %v", err)
		}
		if cfg.ICMPFloodFilter && cfg.ICMPThreshold != "" {
			if err := mainFrame.Locator("input#icmpThreshold").Fill(cfg.ICMPThreshold); err != nil {
				log.Printf("could not fill ICMP threshold: %v", err)
			}
		}

		// 8. Configure UDP-Flood filter
		fmt.Printf("Setting UDP-Flood filter=%v, threshold=%s...\n", cfg.UDPFloodFilter, cfg.UDPThreshold)
		if err := mainFrame.Locator("input#udpFilter").SetChecked(cfg.UDPFloodFilter); err != nil {
			log.Printf("could not set UDP filter: %v", err)
		}
		if cfg.UDPFloodFilter && cfg.UDPThreshold != "" {
			if err := mainFrame.Locator("input#udpThreshold").Fill(cfg.UDPThreshold); err != nil {
				log.Printf("could not fill UDP threshold: %v", err)
			}
		}

		// 9. Configure TCP-SYN-Flood filter
		fmt.Printf("Setting SYN-Flood filter=%v, threshold=%s...\n", cfg.SYNFloodFilter, cfg.SYNThreshold)
		if err := mainFrame.Locator("input#synFilter").SetChecked(cfg.SYNFloodFilter); err != nil {
			log.Printf("could not set SYN filter: %v", err)
		}
		if cfg.SYNFloodFilter && cfg.SYNThreshold != "" {
			if err := mainFrame.Locator("input#synThreshold").Fill(cfg.SYNThreshold); err != nil {
				log.Printf("could not fill SYN threshold: %v", err)
			}
		}
	}

	// 10. Configure Ping filters (independent of DoS Protection toggle)
	fmt.Printf("Setting WAN Ping filter=%v...\n", cfg.WANPingFilter)
	if err := mainFrame.Locator("input#wanPingFilter").SetChecked(cfg.WANPingFilter); err != nil {
		log.Printf("could not set WAN Ping filter: %v", err)
	}

	fmt.Printf("Setting LAN Ping filter=%v...\n", cfg.LANPingFilter)
	if err := mainFrame.Locator("input#lanPingFilter").SetChecked(cfg.LANPingFilter); err != nil {
		log.Printf("could not set LAN Ping filter: %v", err)
	}

	// 11. Save the settings
	fmt.Println("Clicking Save button...")
	saveBtn := mainFrame.Locator("input#save").First()

	// Capture any confirmation dialogs (e.g. reboot prompt)
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
