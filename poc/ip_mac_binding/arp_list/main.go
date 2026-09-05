package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

// ArpEntry represents a single entry in the ARP list
// (IP & MAC バインディング > ARP リスト).
type ArpEntry struct {
	// MACAddress is the MAC address of the device (MAC アドレス).
	MACAddress string

	// IPAddress is the IP address of the device (IP アドレス).
	IPAddress string

	// Status is the current status (ステータス), e.g. "読み込み" (Loaded).
	Status string

	// Imported indicates whether this entry should be imported (選択して読み込み).
	Imported bool
}

func main() {
	// --- Target configuration ---
	// The ARP list is populated by the router from its ARP table (learned
	// devices on the LAN). Here we only read the list and demonstrate the
	// two operations offered by the page:
	//   1. 選択したARPを読み込み (Import selected ARP entries into the
	//      IP-MAC binding list).
	//   2. 選択したエントリを削除 (Delete selected entries from the ARP list).
	importFirstN := 1   // Number of entries to import (選択して読み込み)
	deleteFirstN := 1   // Number of entries to delete (選択して削除)
	dryRun := true      // Set to false to actually perform import/delete

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

	// 4. Find bottomLeftFrame and navigate to IP & MAC Binding > ARP リスト
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

	fmt.Println("Clicking ARP List submenu...")
	arpLoc := leftFrame.Locator("a:has-text('ARP リスト'), a:has-text('ARP 一覧'), a:has-text('ARP List'), a:has-text('ARP Tablet')").First()
	if err := arpLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for ARP List submenu: %v", err)
	}
	if err := arpLoc.Click(); err != nil {
		log.Fatalf("could not click ARP List submenu: %v", err)
	}
	time.Sleep(1 * time.Second)

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

	// 6. Wait for the ARP table and read it (読み出し)
	fmt.Println("Waiting for ARP list table...")
	if err := mainFrame.Locator("table#arptbl").WaitFor(); err != nil {
		log.Fatalf("could not wait for ARP list table: %v", err)
	}
	time.Sleep(1 * time.Second)

	entries, err := readArpTable(mainFrame)
	if err != nil {
		log.Fatalf("could not read ARP table: %v", err)
	}

	fmt.Println()
	fmt.Printf("Found %d ARP entries:\n", len(entries))
	fmt.Println("MAC Address\tIP Address\tStatus")
	for _, e := range entries {
		fmt.Printf("%s\t%s\t%s\n", e.MACAddress, e.IPAddress, e.Status)
	}

	if len(entries) == 0 {
		fmt.Println("ARP list is empty; nothing to import or delete.")
		fmt.Println("PoC Finished")
		return
	}

	// 7. Import selected ARP entries (選択したARPを読み込み)
	if importFirstN > 0 && !dryRun {
		n := importFirstN
		if n > len(entries) {
			n = len(entries)
		}
		fmt.Printf("\nSelecting and importing first %d ARP entry(s)...\n", n)
		if err := checkRows(mainFrame, n); err != nil {
			log.Fatalf("could not select ARP rows: %v", err)
		}

		importBtn := mainFrame.Locator("input#importBtn, input[value*='読み込み'], input[value*='Import']").First()
		if err := importBtn.Click(); err != nil {
			log.Fatalf("could not click import button: %v", err)
		}
		time.Sleep(2 * time.Second)
	} else if importFirstN > 0 {
		fmt.Println("\n[dryRun] Skipping import of ARP entries.")
	}

	// 8. Delete selected ARP entries (選択したエントリを削除)
	if deleteFirstN > 0 && !dryRun {
		n := deleteFirstN
		if n > len(entries) {
			n = len(entries)
		}
		fmt.Printf("Selecting and deleting first %d ARP entry(s)...\n", n)
		if err := checkRows(mainFrame, n); err != nil {
			log.Fatalf("could not select ARP rows: %v", err)
		}

		delBtn := mainFrame.Locator("input.T_delsel, input[value*='削除'], input[value*='Delete']").First()
		if err := delBtn.Click(); err != nil {
			log.Fatalf("could not click delete button: %v", err)
		}
		time.Sleep(2 * time.Second)
	} else if deleteFirstN > 0 {
		fmt.Println("[dryRun] Skipping delete of ARP entries.")
	}

	// 9. Refresh the list (更新) and re-read
	fmt.Println("\nRefreshing ARP list...")
	refreshBtn := mainFrame.Locator("input.T_refresh, input[value='更新'], input[value='Refresh']").First()
	if err := refreshBtn.Click(); err != nil {
		log.Printf("could not click refresh button: %v", err)
	}
	time.Sleep(2 * time.Second)

	entries, err = readArpTable(mainFrame)
	if err != nil {
		log.Fatalf("could not re-read ARP table: %v", err)
	}

	fmt.Printf("ARP list after refresh: %d entries\n", len(entries))
	fmt.Println("MAC Address\tIP Address\tStatus")
	for _, e := range entries {
		fmt.Printf("%s\t%s\t%s\n", e.MACAddress, e.IPAddress, e.Status)
	}

	fmt.Println("PoC Finished")
}

// readArpTable reads all rows from the ARP binding table (table#arptbl).
func readArpTable(frame playwright.Frame) ([]ArpEntry, error) {
	rows, _ := frame.Locator("table#arptbl tr").All()
	var entries []ArpEntry
	for _, row := range rows {
		cols, _ := row.Locator("td").AllInnerTexts()
		// Columns: checkbox column text (empty), MAC, IP, Status
		if len(cols) >= 3 {
			entries = append(entries, ArpEntry{
				MACAddress: strings.TrimSpace(cols[0]),
				IPAddress:  strings.TrimSpace(cols[1]),
				Status:     strings.TrimSpace(cols[2]),
			})
		}
	}
	return entries, nil
}

// checkRows checks the checkbox of the first n data rows in the ARP table.
func checkRows(frame playwright.Frame, n int) error {
	rows, _ := frame.Locator("table#arptbl tr").All()
	count := 0
	for _, row := range rows {
		cb := row.Locator("input[type='checkbox']").First()
		if cnt, _ := cb.Count(); cnt == 0 {
			continue
		}
		if err := cb.Check(); err != nil {
			return fmt.Errorf("could not check row %d checkbox: %v", count+1, err)
		}
		count++
		if count >= n {
			break
		}
	}
	if count == 0 {
		return fmt.Errorf("no checkboxes found in ARP table")
	}
	return nil
}
