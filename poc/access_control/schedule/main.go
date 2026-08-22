package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings for Access Control Schedule (アクセス制御 > スケジュール) entries.
type Config struct {
	// Schedules is the list of schedule entries to manage.
	Schedules []Schedule
}

// Schedule represents a single schedule entry.
type Schedule struct {
	// Description is a human-readable name for the schedule (max 15 chars).
	Description string

	// WeekDay: "day" (毎日) or "week" (毎週)
	WeekDay string

	// Start time (0-47): 0=00:00, 1=00:30, ..., 47=23:30
	StartTime int

	// End time (0-47): 0=00:30, 1=01:00, ..., 47=24:00
	EndTime int

	// WeekDays to apply when WeekDay is "week" (e.g. ["mon", "tue", "wed"])
	WeekDays []string
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		Schedules: []Schedule{
			{
				Description: "WorkHours",
				WeekDay:     "day",
				StartTime:   16, // 08:00
				EndTime:     35, // 17:30
			},
			{
				Description: "WeekendOnly",
				WeekDay:     "week",
				StartTime:   0,  // 00:00
				EndTime:     47, // 24:00
				WeekDays:    []string{"sat", "sun"},
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

	// 4. Find bottomLeftFrame and navigate to Access Control > Schedule
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

	fmt.Println("Clicking Schedule submenu...")
	schedLoc := leftFrame.Locator("a:has-text('スケジュール'), a:has-text('Schedule')").First()
	if err := schedLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Schedule link: %v", err)
	}
	if err := schedLoc.Click(); err != nil {
		log.Fatalf("could not click Schedule submenu: %v", err)
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

	fmt.Println("Waiting for schedule list page...")
	if err := mainFrame.Locator("#tasktbl, table#tasktbl").WaitFor(); err != nil {
		log.Printf("wait for schedule table: %v", err)
	}

	// Handle dialog confirmations
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	// 6. Add schedule entries
	for i, sched := range cfg.Schedules {
		fmt.Printf("Adding schedule %d: %s...\n", i+1, sched.Description)

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
		if err := descInput.Fill(sched.Description); err != nil {
			log.Fatalf("could not fill description: %v", err)
		}

		// Select WeekDay mode
		weekDaySelect := mainFrame.Locator("select#weekDay").First()
		if err := weekDaySelect.WaitFor(); err != nil {
			log.Fatalf("could not wait for weekday select: %v", err)
		}

		if sched.WeekDay == "week" {
			// Select "毎週" (Every week)
			fmt.Println("Selecting weekly mode...")
			if _, err := weekDaySelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"week"},
			}); err != nil {
				log.Fatalf("could not select week mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)

			// Check weekday checkboxes
			for _, day := range sched.WeekDays {
				fmt.Printf("  Checking %s...\n", day)
				dayChk := mainFrame.Locator(fmt.Sprintf("input#%s", day)).First()
				if err := dayChk.SetChecked(true); err != nil {
					log.Fatalf("could not check %s: %v", day, err)
				}
			}
		} else {
			// Select "毎日" (Every day)
			fmt.Println("Selecting daily mode...")
			if _, err := weekDaySelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"day"},
			}); err != nil {
				log.Fatalf("could not select day mode: %v", err)
			}
			time.Sleep(500 * time.Millisecond)
		}

		// Set Start Time
		fmt.Printf("Setting start time: %d...\n", sched.StartTime)
		startTimeSelect := mainFrame.Locator("select#timeS").First()
		if err := startTimeSelect.WaitFor(); err != nil {
			log.Fatalf("could not wait for start time select: %v", err)
		}
		if _, err := startTimeSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{fmt.Sprintf("%d", sched.StartTime)},
		}); err != nil {
			log.Fatalf("could not select start time: %v", err)
		}

		// Set End Time
		fmt.Printf("Setting end time: %d...\n", sched.EndTime)
		endTimeSelect := mainFrame.Locator("select#timeE").First()
		if err := endTimeSelect.WaitFor(); err != nil {
			log.Fatalf("could not wait for end time select: %v", err)
		}
		if _, err := endTimeSelect.SelectOption(playwright.SelectOptionValues{
			Values: &[]string{fmt.Sprintf("%d", sched.EndTime)},
		}); err != nil {
			log.Fatalf("could not select end time: %v", err)
		}

		// Click Add button to add time slot
		fmt.Println("Adding time slot...")
		addTimeBtn := mainFrame.Locator("input[onclick='addTime();'], input.T_add, input[value='追加'], input[value='Add']").First()
		if err := addTimeBtn.Click(); err != nil {
			log.Fatalf("could not click add time button: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Click Save
		fmt.Printf("Saving schedule %d...\n", i+1)
		saveBtn := mainFrame.Locator("input#saveBtn, input.T_save, input[value='保存'], input[value='Save']").First()
		if err := saveBtn.Click(); err != nil {
			log.Fatalf("could not click save button: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	fmt.Println("PoC Finished")
}
