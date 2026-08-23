package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// GlobalSettings holds the global Bandwidth Control settings
// (帯域幅制御パラメーター).
type GlobalSettings struct {
	// Enabled enables Bandwidth Control (帯域幅制御を有効にする).
	Enabled bool

	// LinkType is the line type (回線タイプ):
	//   "adsl"  -> ADSL
	//   "other" -> その他
	LinkType string

	// UploadBandwidth is the total upstream bandwidth in Kbps (送信帯域幅).
	UploadBandwidth string

	// DownloadBandwidth is the total downstream bandwidth in Kbps (受信帯域幅).
	DownloadBandwidth string

	// VoIPEnabled enables VoIP bandwidth guarantee (VoIP帯域幅保証を有効にする).
	VoIPEnabled bool

	// IPTVEnabled enables IPTV bandwidth guarantee (IPTV帯域幅保証を有効にする).
	IPTVEnabled bool

	// IPTVUpMinBW is the guaranteed upstream bandwidth for IPTV in Kbps.
	IPTVUpMinBW string

	// IPTVDownMinBW is the guaranteed downstream bandwidth for IPTV in Kbps.
	IPTVDownMinBW string
}

// TCRule represents a single Bandwidth Control rule entry (帯域幅制御ルール).
type TCRule struct {
	// Enabled determines whether the rule is enabled (ステータス).
	Enabled bool

	// IPStart is the start of the IP range (IP 範囲).
	IPStart string

	// IPEnd is the end of the IP range (IP 範囲).
	IPEnd string

	// PortStart is the start of the port range (ポート範囲). Use "" for blank.
	PortStart string

	// PortEnd is the end of the port range (ポート範囲). Use "" for blank.
	PortEnd string

	// Protocol is the protocol (プロトコル):
	//   "0"  -> すべて
	//   "6"  -> TCP
	//   "17" -> UDP
	Protocol string

	// Priority is the priority from 1 (highest) to 8 (優先度).
	Priority string

	// UpMinBW is the minimum egress bandwidth in Kbps.
	UpMinBW string

	// UpMaxBW is the maximum egress bandwidth in Kbps.
	UpMaxBW string

	// DownMinBW is the minimum ingress bandwidth in Kbps.
	DownMinBW string

	// DownMaxBW is the maximum ingress bandwidth in Kbps.
	DownMaxBW string
}

func main() {
	// --- Target configuration ---
	globals := GlobalSettings{
		Enabled:           true,
		LinkType:          "other",
		UploadBandwidth:   "100000",
		DownloadBandwidth: "200000",
		VoIPEnabled:       false,
		IPTVEnabled:       false,
	}

	rules := []TCRule{
		{
			Enabled:   true,
			IPStart:   "192.168.1.100",
			IPEnd:     "192.168.1.150",
			PortStart: "",
			PortEnd:   "",
			Protocol:  "0", // すべて
			Priority:  "5",
			UpMinBW:   "1024",
			UpMaxBW:   "50000",
			DownMinBW: "2048",
			DownMaxBW: "100000",
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

	// 4. Find bottomLeftFrame and navigate to Bandwidth Control
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

	fmt.Println("Clicking Bandwidth Control menu...")
	bwLoc := leftFrame.Locator("a:has-text('帯域幅制御'), a:has-text('Bandwidth Control')").First()
	if err := bwLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Bandwidth Control menu: %v", err)
	}
	if err := bwLoc.Click(); err != nil {
		log.Fatalf("could not click Bandwidth Control menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Some firmwares expose a submenu with the same name; click it when found.
	subLoc := leftFrame.Locator("a:has-text('帯域幅制御'), a:has-text('Bandwidth Control')").Nth(1)
	if err := subLoc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(2000),
	}); err == nil {
		fmt.Println("Clicking Bandwidth Control submenu...")
		if err := subLoc.Click(); err != nil {
			log.Printf("could not click Bandwidth Control submenu: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	// 5. Find mainFrame and configure global settings
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

	fmt.Println("Waiting for bandwidth control settings...")
	if err := mainFrame.Locator("input#enableTc").WaitFor(); err != nil {
		log.Fatalf("could not wait for enableTc checkbox: %v", err)
	}

	// Enable/disable Bandwidth Control
	if globals.Enabled {
		if err := mainFrame.Locator("input#enableTc").Check(); err != nil {
			log.Fatalf("could not check enableTc: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#enableTc").Uncheck(); err != nil {
			log.Printf("could not uncheck enableTc: %v", err)
		}
	}
	time.Sleep(500 * time.Millisecond)

	// Configure the rest only when enabled (fields may be disabled otherwise)
	if globals.Enabled {
		// Line type
		switch strings.ToLower(globals.LinkType) {
		case "adsl":
			_ = mainFrame.Locator("input#adsl_type").Check()
		default:
			_ = mainFrame.Locator("input#other_type").Check()
		}
		time.Sleep(300 * time.Millisecond)

		// Total upload/download bandwidth
		upInput := mainFrame.Locator("input#upTotalBW").First()
		if err := upInput.Fill(globals.UploadBandwidth); err != nil {
			log.Fatalf("could not fill upload bandwidth: %v", err)
		}
		downInput := mainFrame.Locator("input#downTotalBW").First()
		if err := downInput.Fill(globals.DownloadBandwidth); err != nil {
			log.Fatalf("could not fill download bandwidth: %v", err)
		}

		// VoIP guarantee (only present on some models)
		voipCb := mainFrame.Locator("input#enableVoIPTc")
		if cnt, _ := voipCb.Count(); cnt > 0 {
			if globals.VoIPEnabled {
				_ = voipCb.Check()
			} else {
				_ = voipCb.Uncheck()
			}
		}

		// IPTV guarantee (only present on some models)
		iptvCb := mainFrame.Locator("input#enableIptvTc")
		if cnt, _ := iptvCb.Count(); cnt > 0 {
			if globals.IPTVEnabled {
				_ = iptvCb.Check()
				time.Sleep(300 * time.Millisecond)
				_ = mainFrame.Locator("input#iptvUpMinBW").Fill(globals.IPTVUpMinBW)
				_ = mainFrame.Locator("input#iptvDownMinBW").Fill(globals.IPTVDownMinBW)
			} else {
				_ = iptvCb.Uncheck()
			}
		}

		// Save global settings
		fmt.Println("Saving global settings...")
		saveBtn := mainFrame.Locator("input#saveBtn").First()
		if err := saveBtn.Click(); err != nil {
			log.Fatalf("could not click save button: %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	// 6. Add bandwidth control rules
	for i, rule := range rules {
		fmt.Printf("Adding rule %d (%s-%s)...\n", i+1, rule.IPStart, rule.IPEnd)

		addBtn := mainFrame.Locator("input.T_addnew, input[value='新規追加'], input[value='Add New']").First()
		if err := addBtn.Click(); err != nil {
			log.Fatalf("could not click Add New button: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Enabled checkbox
		if rule.Enabled {
			if err := mainFrame.Locator("input#tc_en").Check(); err != nil {
				log.Printf("could not check tc_en: %v", err)
			}
		} else {
			_ = mainFrame.Locator("input#tc_en").Uncheck()
		}

		// IP range
		if err := mainFrame.Locator("input#ipStart").Fill(rule.IPStart); err != nil {
			log.Fatalf("could not fill ipStart: %v", err)
		}
		if err := mainFrame.Locator("input#ipEnd").Fill(rule.IPEnd); err != nil {
			log.Fatalf("could not fill ipEnd: %v", err)
		}

		// Port range (optional)
		if rule.PortStart != "" && rule.PortEnd != "" {
			if err := mainFrame.Locator("input#portStart").Fill(rule.PortStart); err != nil {
				log.Fatalf("could not fill portStart: %v", err)
			}
			if err := mainFrame.Locator("input#portEnd").Fill(rule.PortEnd); err != nil {
				log.Fatalf("could not fill portEnd: %v", err)
			}
		}

		// Protocol
		if _, err := mainFrame.Locator("select#protocol").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{rule.Protocol},
		}); err != nil {
			log.Fatalf("could not select protocol: %v", err)
		}

		// Priority (1-8)
		if _, err := mainFrame.Locator("select#precedence").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{rule.Priority},
		}); err != nil {
			log.Fatalf("could not select precedence: %v", err)
		}

		// Egress bandwidth (min/max)
		if err := mainFrame.Locator("input#upMinBW").Fill(rule.UpMinBW); err != nil {
			log.Fatalf("could not fill upMinBW: %v", err)
		}
		if err := mainFrame.Locator("input#upMaxBW").Fill(rule.UpMaxBW); err != nil {
			log.Fatalf("could not fill upMaxBW: %v", err)
		}

		// Ingress bandwidth (min/max)
		if err := mainFrame.Locator("input#downMinBW").Fill(rule.DownMinBW); err != nil {
			log.Fatalf("could not fill downMinBW: %v", err)
		}
		if err := mainFrame.Locator("input#downMaxBW").Fill(rule.DownMaxBW); err != nil {
			log.Fatalf("could not fill downMaxBW: %v", err)
		}

		// Save the rule
		fmt.Printf("Saving rule %d...\n", i+1)
		saveBtn := mainFrame.Locator("input#saveBtn, input.T_save").First()
		if err := saveBtn.Click(); err != nil {
			log.Fatalf("could not click save button: %v", err)
		}
		time.Sleep(2 * time.Second)

		// Go back to the rule list
		backBtn := mainFrame.Locator("input.T_back").First()
		if err := backBtn.Click(); err != nil {
			log.Printf("could not click back button (maybe already back): %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	// 7. Read the resulting rule list
	fmt.Println("Reading Bandwidth Control rule list...")
	rows, _ := mainFrame.Locator("table#tctbl tr, .tbody table tr").All()
	for _, row := range rows {
		cols, _ := row.Locator("td").AllInnerTexts()
		if len(cols) > 0 {
			fmt.Println(strings.Join(cols, "\t"))
		}
	}

	fmt.Println("PoC Finished")
}
