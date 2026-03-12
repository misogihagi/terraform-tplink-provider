package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
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
		Headless: playwright.Bool(false), // false でブラウザの動作を目視確認
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	// === 設定値 ===
	endpoint := "http://192.168.1.1"
	username := "admin"
	password := "admin"
	newSSID := "TP-Link_SSID_PoC"
	// モード値: "11bgn mixed" = "13", "11bg mixed" = "10", "11b only" = "2", "11g only" = "3", "11n only (HT20)" = "8"
	newMode := "n"
	// チャンネル: "0" = 自動, "1"〜"13" = 各チャンネル番号
	newChannel := "0"
	// チャンネル幅: "Auto" = 自動, "20M" = 20MHz, "40M" = 40MHz
	newChannelWidth := "Auto"
	// SSID ブロードキャスト: true = 有効, false = 無効
	ssidBroadcast := true

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 2. ログイン
	fmt.Println("Attempting login...")
	if err := page.Locator("#userName").WaitFor(); err != nil {
		log.Fatalf("could not wait for username input: %v", err)
	}
	if err := page.Locator("#userName").Fill(username); err != nil {
		log.Fatalf("could not fill username: %v", err)
	}
	if err := page.Locator("#pcPassword").Fill(password); err != nil {
		log.Fatalf("could not fill password: %v", err)
	}
	if err := page.Locator("#loginBtn").Click(); err != nil {
		log.Fatalf("could not click login: %v", err)
	}

	if err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// 3. bottomLeftFrame を検索
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

	// 4. 「ワイヤレス 2.4GHz」メニューをクリック
	fmt.Println("Clicking Wireless 2.4GHz menu...")
	wireless24Loc := leftFrame.Locator("a:has-text('ワイヤレス 2.4GHz'), a:has-text('Wireless 2.4GHz')").First()
	if err := wireless24Loc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Wireless 2.4GHz menu: %v", err)
	}
	if err := wireless24Loc.Click(); err != nil {
		log.Fatalf("could not click Wireless 2.4GHz menu: %v", err)
	}
	time.Sleep(1 * time.Second)

	// 5. 「基本設定」サブメニューをクリック
	fmt.Println("Clicking 基本設定 submenu...")
	basicSettingsLoc := leftFrame.Locator("a:has-text('基本設定'), a:has-text('Basic Settings'), a:has-text('Wireless Settings')").First()
	if err := basicSettingsLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for 基本設定 link: %v", err)
	}
	if err := basicSettingsLoc.Click(); err != nil {
		log.Fatalf("could not click 基本設定 submenu: %v", err)
	}

	// 6. mainFrame を検索
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

	// 7. SSID フィールドに入力
	fmt.Printf("Filling SSID: %s\n", newSSID)
	ssidInput := mainFrame.Locator("input#ssid").First()
	if err := ssidInput.WaitFor(); err != nil {
		log.Fatalf("could not wait for SSID input: %v", err)
	}
	if err := ssidInput.Fill(newSSID); err != nil {
		log.Fatalf("could not fill SSID: %v", err)
	}

	// 8. モード選択
	fmt.Printf("Selecting Mode: %s\n", newMode)
	modeSelect := mainFrame.Locator("select#mode").First()
	if _, err := modeSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{newMode}}); err != nil {
		log.Printf("could not select mode (skipping): %v", err)
	}

	// 9. チャンネル選択
	fmt.Printf("Selecting Channel: %s\n", newChannel)
	channelSelect := mainFrame.Locator("select#channel").First()
	if _, err := channelSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{newChannel}}); err != nil {
		log.Printf("could not select channel (skipping): %v", err)
	}

	// 10. チャンネル幅選択
	fmt.Printf("Selecting Channel Width: %s\n", newChannelWidth)
	chanBWSelect := mainFrame.Locator("select#bandWidth").First()
	if _, err := chanBWSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{newChannelWidth}}); err != nil {
		log.Printf("could not select channel width (skipping): %v", err)
	}

	// 11. SSID ブロードキャスト
	broadcastCheckbox := mainFrame.Locator("input#ssidBroadcast").First()
	isChecked, err := broadcastCheckbox.IsChecked()
	if err != nil {
		log.Printf("could not check broadcast status (skipping): %v", err)
	} else {
		if ssidBroadcast && !isChecked {
			fmt.Println("Enabling SSID broadcast...")
			_ = broadcastCheckbox.Check()
		} else if !ssidBroadcast && isChecked {
			fmt.Println("Disabling SSID broadcast...")
			_ = broadcastCheckbox.Uncheck()
		} else {
			fmt.Println("SSID broadcast already in desired state.")
		}
	}

	// 12. 保存
	fmt.Println("Saving settings...")
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})
	saveBtn := mainFrame.Locator("input.T_save").First()
	if err := saveBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for Save button: %v", err)
	}
	if err := saveBtn.Click(); err != nil {
		log.Fatalf("could not click Save button: %v", err)
	}

	fmt.Println("Waiting for router to apply settings (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("\nPoC Finished successfully.")
}
