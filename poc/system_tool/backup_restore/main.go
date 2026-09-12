package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
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
	backupFile := "/tmp/config.bin"
	restoreFile := "/tmp/config.bin"

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

	// Click Backup & Restore submenu
	fmt.Println("Clicking Backup & Restore submenu...")
	backupRestoreLoc := leftFrame.Locator("a:has-text('バックアップ & 復元'), a:has-text('Backup & Restore'), a:has-text('Backup/Restore')").First()
	if err := backupRestoreLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Backup & Restore submenu: %v", err)
	}
	_ = backupRestoreLoc.Click()
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

	// Wait for backup & restore page
	if err := mainFrame.Locator("#t_backup").WaitFor(); err != nil {
		log.Fatalf("could not wait for backup & restore page: %v", err)
	}

	// Read info text
	info, err := mainFrame.Locator("#t_info2").InnerText()
	if err != nil {
		log.Printf("could not read info text: %v", err)
	} else {
		fmt.Printf("Info text: %s\n", info)
	}

	// Backup: click Backup button and save downloaded config file
	fmt.Println("Clicking Backup button...")
	download, err := page.ExpectDownload(func() error {
		return mainFrame.Locator("#t_backup").Click()
	}, playwright.PageExpectDownloadOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		log.Fatalf("could not download config: %v", err)
	}
	filename := download.SuggestedFilename()
	fmt.Printf("Downloaded config: %s\n", filename)
	if err := download.SaveAs(backupFile); err != nil {
		log.Fatalf("could not save config: %v", err)
	}
	fmt.Printf("Config saved to %s\n", backupFile)

	// Restore: select config file and click Restore button
	fmt.Printf("Selecting config file for restore: %s\n", restoreFile)
	if err := mainFrame.Locator("#filename").SetInputFiles(restoreFile); err != nil {
		log.Fatalf("could not select config file: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Handle restore confirmation dialog
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	fmt.Println("Clicking Restore button...")
	_ = mainFrame.Locator("#t_restore").Click()

	// Some models prompt for the admin password before restoring
	if err := page.Locator("#pcPassword").WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(3000),
	}); err == nil {
		_ = page.Locator("#pcPassword").Fill(password)
		_ = page.Locator("#confirmBtn").Click()
	}

	// Wait for router to apply the restored config
	fmt.Println("Waiting for router to apply restored config (60s)...")
	time.Sleep(60 * time.Second)

	fmt.Println("PoC Finished.")
}