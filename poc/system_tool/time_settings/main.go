package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
	// 1. Playwright Setup
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
		Headless: playwright.Bool(false), // Show browser for PoC visibility
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	// === Configuration Values ===
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"

	// Time Settings
	timezone := "+09:00"         // GMT+9 (Tokyo)
	year := "2026"
	month := "09"
	day := "10"
	hour := "15"
	minute := "30"
	second := "00"
	ntpA := "ntp.nict.jp"       // NTP Server 1
	ntpB := "time.asia.apple.com" // NTP Server 2

	// DST Settings
	dstEnable := true
	dstStartMonth := "03"
	dstStartWeekCount := "5"    // Last
	dstStartWeekDay := "7"      // Sunday
	dstStartTime := "02:00:00"
	dstEndMonth := "11"
	dstEndWeekCount := "1"      // First
	dstEndWeekDay := "7"        // Sunday
	dstEndTime := "02:00:00"

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 2. Login
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

	// 3. Navigate to System Tools > Time Settings menu
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

	fmt.Println("Clicking System Tools menu...")
	systemToolsMenuLoc := leftFrame.Locator("a:has-text('システムツール'), a:has-text('System Tools')").First()
	if err := systemToolsMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for System Tools menu: %v", err)
	}
	_ = systemToolsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking Time Settings submenu...")
	timeSettingsMenuLoc := leftFrame.Locator("a:has-text('時刻設定'), a:has-text('Time Settings')").First()
	if err := timeSettingsMenuLoc.WaitFor(); err == nil {
		_ = timeSettingsMenuLoc.Click()
		time.Sleep(1 * time.Second)
	}

	// 4. Interacting with settings (mainFrame)
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

	// Wait for the page to render
	if err := mainFrame.Locator("select#timezone").WaitFor(); err != nil {
		log.Fatalf("could not wait for Time Settings page to load: %v", err)
	}

	// Select Timezone
	fmt.Printf("Selecting timezone: %s\n", timezone)
	_, _ = mainFrame.Locator("select#timezone").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(timezone),
	})
	time.Sleep(1 * time.Second)

	// Set Date
	fmt.Printf("Setting date: %s/%s/%s\n", year, month, day)
	_ = mainFrame.Locator("input#year").Fill(year)
	_ = mainFrame.Locator("input#month").Fill(month)
	_ = mainFrame.Locator("input#day").Fill(day)

	// Set Time
	fmt.Printf("Setting time: %s:%s:%s\n", hour, minute, second)
	_ = mainFrame.Locator("input#hour").Fill(hour)
	_ = mainFrame.Locator("input#minute").Fill(minute)
	_ = mainFrame.Locator("input#second").Fill(second)

	// Set NTP Servers
	if ntpA != "" {
		fmt.Printf("Setting NTP Server 1: %s\n", ntpA)
		_ = mainFrame.Locator("input#ntpA").Fill(ntpA)
	}
	if ntpB != "" {
		fmt.Printf("Setting NTP Server 2: %s\n", ntpB)
		_ = mainFrame.Locator("input#ntpB").Fill(ntpB)
	}

	// 5. Handle DST Settings
	fmt.Println("Configuring DST settings...")
	if dstEnable {
		fmt.Println("Enabling DST...")
		_ = mainFrame.Locator("input#enableDST").Check()
		time.Sleep(1 * time.Second)

		// DST Start
		fmt.Println("Setting DST Start...")
		_, _ = mainFrame.Locator("select#dst_start_month").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstStartMonth),
		})
		_, _ = mainFrame.Locator("select#dst_start_weekCount").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstStartWeekCount),
		})
		_, _ = mainFrame.Locator("select#dst_start_weekDay").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstStartWeekDay),
		})
		_, _ = mainFrame.Locator("select#dst_start_time").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstStartTime),
		})

		// DST End
		fmt.Println("Setting DST End...")
		_, _ = mainFrame.Locator("select#dst_end_month").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstEndMonth),
		})
		_, _ = mainFrame.Locator("select#dst_end_weekCount").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstEndWeekCount),
		})
		_, _ = mainFrame.Locator("select#dst_end_weekDay").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstEndWeekDay),
		})
		_, _ = mainFrame.Locator("select#dst_end_time").SelectOption(playwright.SelectOptionValues{
			Values: playwright.StringSlice(dstEndTime),
		})

		// Save DST
		fmt.Println("Saving DST settings...")
		dstSaveBtn := mainFrame.Locator("input#saveDSTBtn").First()
		if err := dstSaveBtn.WaitFor(); err != nil {
			log.Fatalf("could not wait for DST save button: %v", err)
		}
		_ = dstSaveBtn.Click()
		time.Sleep(3 * time.Second)
	}

	// 6. Save Time Settings
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	fmt.Println("Saving time settings...")
	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	fmt.Println("Waiting for router to apply (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished.")
}
