package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings to apply to the Parental Controls (保護者による制限) page.
type Config struct {
	// Enabled enables or disables Parental Controls.
	Enabled bool

	// ParentMac is the MAC address of the parent's PC.
	// Leave empty to copy the current PC's MAC with the "上にコピー" button.
	ParentMac string

	// ManagedMacs are the MAC addresses of the PCs whose access is restricted (up to 4).
	ManagedMacs []string

	// ScheduleStart is the start time value (00:00-23:30, i.e. option value 0-47).
	// ScheduleEnd is the end time value (00:30-24:00, i.e. option value 0-47).
	ScheduleStart string
	ScheduleEnd   string

	// Weekly restricts only on the selected weekdays (Mon..Sun). Requires weekDay select = "week".
	Weekly   bool
	WeekDays []string // e.g. ["mon", "tue"]

	// BlockedUrls are websites blocked by Parental Controls.
	BlockedUrls []string
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		Enabled:       true,
		ParentMac:     "AA:BB:CC:DD:EE:FF",
		ManagedMacs:   []string{"11:22:33:44:55:66", ""},
		ScheduleStart: "8",  // 04:00
		ScheduleEnd:   "20", // 10:30
		Weekly:        true,
		WeekDays:      []string{"mon", "tue", "wed"},
		BlockedUrls:   []string{"facebook.com", "youtube.com"},
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

	// 4. Find bottomLeftFrame and navigate to Parental Controls
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

	fmt.Println("Clicking Parental Controls menu...")
	parentalLoc := leftFrame.Locator("a:has-text('保護者による制限'), a:has-text('Parental Controls')").First()
	if err := parentalLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Parental Controls link: %v", err)
	}
	if err := parentalLoc.Click(); err != nil {
		log.Fatalf("could not click Parental Controls menu: %v", err)
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

	fmt.Println("Waiting for Parental Controls enable checkbox...")
	enableChk := mainFrame.Locator("input#ParentCtr_en")
	if err := enableChk.WaitFor(); err != nil {
		log.Fatalf("could not wait for Parental Controls enable checkbox: %v", err)
	}

	// 6. Enable/disable Parental Controls
	fmt.Printf("Setting Parental Controls enabled=%v...\n", cfg.Enabled)
	if err := enableChk.SetChecked(cfg.Enabled); err != nil {
		log.Fatalf("could not set Parental Controls enable: %v", err)
	}

	if cfg.Enabled {
		// 7. Set parent PC's MAC address
		if cfg.ParentMac == "" {
			fmt.Println("No parent MAC specified, using the '上にコピー' button...")
			copyBtn := mainFrame.Locator("input[onclick='setParentMac();'], input[value='上にコピー']").First()
			if err := copyBtn.Click(); err != nil {
				log.Fatalf("could not click copy parent MAC button: %v", err)
			}
		} else {
			fmt.Printf("Setting parent PC MAC address to %s...\n", cfg.ParentMac)
			if err := mainFrame.Locator("input#parentMac").Fill(cfg.ParentMac); err != nil {
				log.Fatalf("could not fill parent MAC: %v", err)
			}
		}

		// 8. Set managed PCs' MAC addresses (mac1..mac4)
		for i, mac := range cfg.ManagedMacs {
			if mac == "" {
				continue
			}
			sel := fmt.Sprintf("input#mac%d", i+1)
			fmt.Printf("Setting managed MAC %d to %s...\n", i+1, mac)
			if err := mainFrame.Locator(sel).Fill(mac); err != nil {
				log.Fatalf("could not fill managed MAC %d: %v", i+1, err)
			}
		}

		// 9. Configure the schedule
		if cfg.Weekly {
			fmt.Println("Setting schedule mode to weekly...")
			if _, err := mainFrame.Locator("select#weekDay").SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"week"},
			}); err != nil {
				log.Fatalf("could not select weekly schedule mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			fmt.Println("Selecting weekdays...")
			for _, d := range cfg.WeekDays {
				if err := mainFrame.Locator("input#" + d).SetChecked(true); err != nil {
					log.Printf("could not check weekday %s: %v", d, err)
				}
			}
		} else {
			fmt.Println("Setting schedule mode to daily...")
			if _, err := mainFrame.Locator("select#weekDay").SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"day"},
			}); err != nil {
				log.Fatalf("could not select daily schedule mode: %v", err)
			}
		}

		fmt.Printf("Setting schedule start=%s end=%s...\n", cfg.ScheduleStart, cfg.ScheduleEnd)
		if _, err := mainFrame.Locator("select#timeS").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{cfg.ScheduleStart},
		}); err != nil {
			log.Fatalf("could not select start time: %v", err)
		}
		if _, err := mainFrame.Locator("select#timeE").SelectOption(playwright.SelectOptionValues{
			Values: &[]string{cfg.ScheduleEnd},
		}); err != nil {
			log.Fatalf("could not select end time: %v", err)
		}

		fmt.Println("Clicking schedule Add button...")
		addTimeBtn := mainFrame.Locator("input[onclick='addTime();']").First()
		if err := addTimeBtn.Click(); err != nil {
			log.Fatalf("could not click schedule add button: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// 10. Block the URLs
		for _, url := range cfg.BlockedUrls {
			fmt.Printf("Adding blocked URL %s...\n", url)
			if err := mainFrame.Locator("input#urlInfo").Fill(url); err != nil {
				log.Fatalf("could not fill URL input: %v", err)
			}
			addUrlBtn := mainFrame.Locator("input[onclick='doAddUrl();']").First()
			if err := addUrlBtn.Click(); err != nil {
				log.Fatalf("could not click URL add button: %v", err)
			}
			time.Sleep(300 * time.Millisecond)
		}
	}

	// 11. Save the settings
	fmt.Println("Clicking Save button...")
	saveBtn := mainFrame.Locator("input#saveBtn").First()

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
