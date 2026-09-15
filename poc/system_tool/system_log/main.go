package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

type LogEntry struct {
	Index   string
	Time    string
	Type    string
	Level   string
	Content string
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

	// Click System Log submenu
	fmt.Println("Clicking System Log submenu...")
	systemLogLoc := leftFrame.Locator("a:has-text('システム ログ'), a:has-text('System Log')").First()
	if err := systemLogLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for System Log submenu: %v", err)
	}
	_ = systemLogLoc.Click()
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

	// Wait for the log table
	fmt.Println("Waiting for log table...")
	if err := mainFrame.Locator("#log_tbl").WaitFor(); err != nil {
		log.Fatalf("could not wait for log table: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Read log entries
	entries, err := readLogTable(mainFrame)
	if err != nil {
		log.Fatalf("could not read log table: %v", err)
	}

	fmt.Printf("\nFound %d log entries:\n", len(entries))
	fmt.Println("Index\tTime\tType\tLevel\tContent")
	for _, e := range entries {
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", e.Index, e.Time, e.Type, e.Level, e.Content)
	}

	fmt.Println("\nPoC Finished.")
}

func readLogTable(frame playwright.Frame) ([]LogEntry, error) {
	rows, err := frame.Locator("#log_tbl tr").All()
	if err != nil {
		return nil, fmt.Errorf("could not read log table rows: %v", err)
	}

	var entries []LogEntry
	for _, row := range rows {
		cols, err := row.Locator("td").AllInnerTexts()
		if err != nil || len(cols) < 5 {
			continue
		}
		entries = append(entries, LogEntry{
			Index:   strings.TrimSpace(cols[0]),
			Time:    strings.TrimSpace(cols[1]),
			Type:    strings.TrimSpace(cols[2]),
			Level:   strings.TrimSpace(cols[3]),
			Content: strings.TrimSpace(cols[4]),
		})
	}

	return entries, nil
}
