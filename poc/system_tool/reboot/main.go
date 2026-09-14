package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

const (
	// Auto-reboot configuration to apply (system tools > reboot).
	// rebootMode: "0" = disabled, "1" = timeout, "2" = schedule.
	rebootMode       = "2"
	scheduleEveryday = true
	scheduleHour     = "3"
	scheduleMinute   = "30"
	timeoutHour      = "1"
	timeoutMinute    = "30"
)

func main() {
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

	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"

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

	// Click Reboot submenu
	fmt.Println("Clicking Reboot submenu...")
	rebootLoc := leftFrame.Locator("a:has-text('再起動'), a:has-text('Reboot')").First()
	if err := rebootLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Reboot submenu: %v", err)
	}
	_ = rebootLoc.Click()
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

	// Wait for the reboot page
	fmt.Println("Waiting for reboot page...")
	if err := mainFrame.Locator("#button_reboot").WaitFor(); err != nil {
		log.Fatalf("could not wait for reboot page: %v", err)
	}

	// Read info text
	info, err := mainFrame.Locator("#t_info").InnerText()
	if err != nil {
		log.Printf("could not read info text: %v", err)
	} else {
		fmt.Printf("Info text: %s\n", info)
	}

	// Read current auto-reboot settings
	readAutoReboot(mainFrame)

	// Configure auto-reboot
	if err := configureAutoReboot(mainFrame); err != nil {
		log.Fatalf("could not configure auto-reboot: %v", err)
	}

	// Handle reboot confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// Click Reboot button
	fmt.Println("Clicking Reboot button...")
	_ = mainFrame.Locator("#button_reboot").Click()

	// Wait for router to restart
	fmt.Println("Waiting for router to restart (90s)...")
	time.Sleep(90 * time.Second)

	// Attempt to reconnect to verify the router is back (login page)
	fmt.Println("Attempting to verify the router is back online...")
	for i := 0; i < 6; i++ {
		if _, err := page.Goto(endpoint); err == nil {
			err := page.Locator("#userName").WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
			})
			if err == nil {
				fmt.Println("Login page is available. Reboot PoC succeeded.")
				break
			}
		}
		time.Sleep(10 * time.Second)
	}

	fmt.Println("PoC Finished.")
}

// readAutoReboot prints the current auto-reboot settings of the router.
func readAutoReboot(mainFrame playwright.Frame) {
	fmt.Println("Reading current auto-reboot settings...")

	mode, err := mainFrame.Locator("#t_autoRebootTime").InputValue()
	if err != nil {
		log.Printf("could not read auto-reboot mode: %v", err)
		return
	}
	modeNames := map[string]string{"0": "disabled", "1": "timeout", "2": "schedule"}
	fmt.Printf("Auto-reboot mode: %s (%s)\n", modeNames[mode], mode)

	everyday, err := mainFrame.Locator("#day_type_all").IsChecked()
	if err == nil {
		days := "specific days"
		if everyday {
			days = "everyday"
		}
		fmt.Printf("Day selection: %s\n", days)
	}

	hour, _ := mainFrame.Locator("#hour_id").InputValue()
	minute, _ := mainFrame.Locator("#minute_id").InputValue()
	fmt.Printf("Schedule time: %s:%s\n", hour, minute)
}

// configureAutoReboot applies the auto-reboot settings defined by the
// constants above and saves them.
func configureAutoReboot(mainFrame playwright.Frame) error {
	fmt.Printf("Configuring auto-reboot (mode=%s)...\n", rebootMode)

	// Select the auto-reboot mode. The onchange handler shows/hides the
	// day/time or timeout rows automatically.
	if _, err := mainFrame.Locator("#t_autoRebootTime").SelectOption(playwright.SelectOptionValues{
		Values: &[]string{rebootMode},
	}); err != nil {
		return fmt.Errorf("could not select auto-reboot mode: %v", err)
	}

	switch rebootMode {
	case "1":
		// Timeout mode: set hours and minutes after which the router reboots.
		if err := mainFrame.Locator("#t_timeouthour").Fill(timeoutHour); err != nil {
			return fmt.Errorf("could not set timeout hour: %v", err)
		}
		if err := mainFrame.Locator("#t_timeoutmin").Fill(timeoutMinute); err != nil {
			return fmt.Errorf("could not set timeout minute: %v", err)
		}
	case "2":
		// Schedule mode: choose every day or specific days.
		if scheduleEveryday {
			if err := mainFrame.Locator("#day_type_all").Check(); err != nil {
				return fmt.Errorf("could not select every day: %v", err)
			}
		} else {
			if err := mainFrame.Locator("#day_type_choose").Check(); err != nil {
				return fmt.Errorf("could not select choose days: %v", err)
			}
			for _, day := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
				if err := mainFrame.Locator("#" + day + "_select").Check(); err != nil {
					return fmt.Errorf("could not check %s: %v", day, err)
				}
			}
		}

		// Set the schedule time (hour:minute).
		if _, err := mainFrame.Locator("#hour_id").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{scheduleHour},
		}); err != nil {
			return fmt.Errorf("could not select schedule hour: %v", err)
		}
		if _, err := mainFrame.Locator("#minute_id").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{scheduleMinute},
		}); err != nil {
			return fmt.Errorf("could not select schedule minute: %v", err)
		}
	}

	// Save the auto-reboot settings.
	fmt.Println("Saving auto-reboot settings...")
	if err := mainFrame.Locator("#Submit").Click(); err != nil {
		return fmt.Errorf("could not click save button: %v", err)
	}
	time.Sleep(1 * time.Second)

	return nil
}