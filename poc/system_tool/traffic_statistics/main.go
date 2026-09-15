package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config for the "set" operation (resource part).
// Set to true to actually apply changes to the router.
const (
	dryRun          = true
	enableStat      = true // true = 有効にする, false = 無効
	statIntervalSec = "30" // 統計間隔 (5-60 seconds)
)

// TrafficStatEntry is a single row in the statistics list (統計リスト).
type TrafficStatEntry struct {
	IPAddress    string
	MACAddress   string
	TotalPackets string
	TotalBytes   string
	CurPackets   string
	CurBytes     string
	ICMPTx       string
	UDPTx        string
	SYNTx        string
}

func main() {
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"

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

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// Login
	fmt.Println("Attempting login...")
	if err := page.Locator("#userName").WaitFor(); err != nil {
		log.Fatalf("could not wait for username input: %v", err)
	}
	_ = page.Locator("#userName").Fill(username)
	_ = page.Locator("#pcPassword").Fill(password)
	_ = page.Locator("#loginBtn").Click()

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// Handle any confirmation dialogs
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// Find bottomLeftFrame
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

	// Click System Tools menu
	fmt.Println("Clicking System Tools menu...")
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// Click Traffic Statistics submenu
	fmt.Println("Clicking Traffic Statistics submenu...")
	trafficStatsLoc := leftFrame.Locator("a:has-text('トラフィック統計'), a:has-text('Traffic Statistics')").First()
	if err := trafficStatsLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Traffic Statistics submenu: %v", err)
	}
	_ = trafficStatsLoc.Click()
	time.Sleep(1 * time.Second)

	// Find mainFrame
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

	// Wait for the traffic statistics page
	if err := mainFrame.Locator("#stat_table").WaitFor(); err != nil {
		log.Fatalf("could not wait for traffic statistics page: %v", err)
	}
	time.Sleep(1 * time.Second)

	// === READ part (data source) ===
	fmt.Println("\n=== READ current settings ===")
	readCurrentSettings(mainFrame)

	fmt.Println("\n=== READ statistics list ===")
	entries, err := readStatTable(mainFrame)
	if err != nil {
		log.Fatalf("could not read statistics table: %v", err)
	}
	if len(entries) == 0 {
		fmt.Println("Statistics list is empty.")
	} else {
		fmt.Printf("Found %d entries:\n", len(entries))
		fmt.Println("IP Address\tMAC Address\tTotal(Pkt)\tTotal(Bytes)\tCur(Pkt)\tCur(Bytes)\tICMP Tx\tUDP Tx\tSYN Tx")
		for _, e := range entries {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				e.IPAddress, e.MACAddress, e.TotalPackets, e.TotalBytes,
				e.CurPackets, e.CurBytes, e.ICMPTx, e.UDPTx, e.SYNTx)
		}
	}

	// === SET part (resource) ===
	if dryRun {
		fmt.Println("\n[dryRun] Skipping apply of stat enable/interval.")
	} else {
		fmt.Println("\n=== SET stat enable & interval ===")
		if err := setStatEnable(mainFrame); err != nil {
			log.Fatalf("could not set traffic statistics: %v", err)
		}
		if err := setStatInterval(mainFrame); err != nil {
			log.Fatalf("could not set stat interval: %v", err)
		}
		time.Sleep(3 * time.Second)

		fmt.Println("\n=== READ after set ===")
		readCurrentSettings(mainFrame)
	}

	fmt.Println("\nPoC Finished.")
}

func readCurrentSettings(frame playwright.Frame) {
	enabled, err := frame.Locator("#stat_en").IsChecked()
	if err == nil {
		fmt.Printf("Traffic statistics enabled: %v\n", enabled)
	}

	intervalVal, err := frame.Locator("#interval").InputValue()
	if err == nil {
		fmt.Printf("Statistics interval: %s seconds\n", intervalVal)
	}
}

func readStatTable(frame playwright.Frame) ([]TrafficStatEntry, error) {
	rows, err := frame.Locator("#stat_table tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read table rows: %v", err)
	}

	var entries []TrafficStatEntry
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 7 {
			continue // skip header/empty rows
		}

		ipMac := strings.TrimSpace(cols[0])
		parts := strings.Split(ipMac, "\n")
		ip := ""
		mac := ""
		if len(parts) >= 1 {
			ip = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			mac = strings.TrimSpace(parts[1])
		}

		entries = append(entries, TrafficStatEntry{
			IPAddress:    ip,
			MACAddress:   mac,
			TotalPackets: strings.TrimSpace(cols[1]),
			TotalBytes:   strings.TrimSpace(cols[2]),
			CurPackets:   strings.TrimSpace(cols[3]),
			CurBytes:     strings.TrimSpace(cols[4]),
			ICMPTx:       strings.TrimSpace(cols[5]),
			UDPTx:        strings.TrimSpace(cols[6]),
			SYNTx:        strings.TrimSpace(cols[7]),
		})
	}

	return entries, nil
}

func setStatEnable(frame playwright.Frame) error {
	target := "#stat_en"
	if !enableStat {
		target = "#stat_dis"
	}

	fmt.Printf("Selecting %s (enable=%v)...\n", target, enableStat)
	loc := frame.Locator(target).First()
	if err := loc.WaitFor(); err != nil {
		return fmt.Errorf("could not find stat radio: %v", err)
	}
	if err := loc.Check(); err != nil {
		return fmt.Errorf("could not select stat radio: %v", err)
	}

	saveBtn := frame.Locator("input:has-text('保存'), input:has-text('Save')").First()
	if err := saveBtn.WaitFor(); err != nil {
		return fmt.Errorf("could not wait for save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	// Re-apply because the page reloads after saving the enable state.
	loc = frame.Locator(target).First()
	if err := loc.WaitFor(); err != nil {
		return fmt.Errorf("could not re-find stat radio after reload: %v", err)
	}
	if err := loc.Check(); err != nil {
		return fmt.Errorf("could not re-select stat radio: %v", err)
	}
	saveBtn = frame.Locator("input:has-text('保存'), input:has-text('Save')").First()
	if err := saveBtn.Click(); err != nil {
		return fmt.Errorf("could not re-click save button: %v", err)
	}
	time.Sleep(2 * time.Second)

	return nil
}

func setStatInterval(frame playwright.Frame) error {
	if _, err := frame.Locator("#interval").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(statIntervalSec),
	}); err != nil {
		return fmt.Errorf("could not select interval: %v", err)
	}
	_ = frame.Locator("#interval").DispatchEvent("change")
	time.Sleep(1 * time.Second)

	// The interval change may require a save as well via changeInterval();
	// re-select the interval and save if a save button is present.
	saveBtn := frame.Locator("input:has-text('保存'), input:has-text('Save')").First()
	if err := saveBtn.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(2000),
	}); err == nil {
		if err := saveBtn.Click(); err != nil {
			return fmt.Errorf("could not click interval save button: %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	return nil
}