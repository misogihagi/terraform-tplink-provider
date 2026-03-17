package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
	// 1. Playwright のセットアップ
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
		Headless: playwright.Bool(false), // 動作確認のためブラウザを表示
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

	// ゲストネットワーク設定
	isEnable := true
	ssid := "Guest_PoC"
	maxStaNum := "8"
	securityType := "1" // 0: 無効, 1: WPA/WPA2 - パーソナル
	pskAuthType := "2"  // 0: 自動, 1: WPA-PSK, 2: WPA2-PSK
	pskCipher := "2"    // 0: 自動, 1: TKIP, 2: AES
	pskSecret := "guestpassword123"

	// 詳細設定
	lanAccess := "0" // 0: 無効, 1: 有効
	wlanIso := "1"   // 0: 無効, 1: 有効

	// 帯域幅制御
	tcEnable := false // ゲストネットワーク帯域幅制御

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 2. ログイン
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

	// 3. ゲストネットワークメニューに移動
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

	fmt.Println("Clicking Guest Network menu...")
	// DOMから推測: ゲストネットワークメニューをクリック
	guestMenuLoc := leftFrame.Locator("a:has-text('ゲスト ネットワーク'), a:has-text('Guest Network')").First()
	if err := guestMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Guest Network menu: %v", err)
	}
	_ = guestMenuLoc.Click()
	time.Sleep(1 * time.Second)

	// 4. 設定の入力 (mainFrame)
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

	// ゲストネットワークの有効/無効
	if isEnable {
		fmt.Println("Enabling Guest Network...")
		_ = mainFrame.Locator("input#guestEn").Check()
	} else {
		fmt.Println("Disabling Guest Network...")
		_ = mainFrame.Locator("input#guestDis").Check()
	}

	// SSIDと最大接続数
	fmt.Printf("Setting SSID: %s, Max Stations: %s\n", ssid, maxStaNum)
	_ = mainFrame.Locator("input#guestSSID").Fill(ssid)
	_ = mainFrame.Locator("input#guestMaxstanum").Fill(maxStaNum)

	// セキュリティ設定
	fmt.Printf("Setting Security Type: %s\n", securityType)
	_, _ = mainFrame.Locator("select#SecurityType").SelectOption(playwright.SelectOptionValues{Values: &[]string{securityType}})

	if securityType == "1" {
		fmt.Println("Configuring WPA/WPA2 Security...")
		_, _ = mainFrame.Locator("select#pskAuthType").SelectOption(playwright.SelectOptionValues{Values: &[]string{pskAuthType}})
		_, _ = mainFrame.Locator("select#pskCipher").SelectOption(playwright.SelectOptionValues{Values: &[]string{pskCipher}})
		_ = mainFrame.Locator("input#pskSecret").Fill(pskSecret)
	}

	// 詳細設定
	fmt.Println("Configuring Access / Isolation...")
	_, _ = mainFrame.Locator("select#lan_access").SelectOption(playwright.SelectOptionValues{Values: &[]string{lanAccess}})
	_, _ = mainFrame.Locator("select#wlan_iso").SelectOption(playwright.SelectOptionValues{Values: &[]string{wlanIso}})

	// 帯域幅制御
	if tcEnable {
		fmt.Println("Enabling Bandwidth Control...")
		_, _ = mainFrame.Locator("select#guestTcEn").SelectOption(playwright.SelectOptionValues{Values: &[]string{"1"}})
		// 必要に応じて BW 入力を追加
	} else {
		_, _ = mainFrame.Locator("select#guestTcEn").SelectOption(playwright.SelectOptionValues{Values: &[]string{"0"}})
	}

	// 5. 保存
	fmt.Println("Saving settings...")
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	saveBtn := mainFrame.Locator("input#saveBtn").First()
	if err := saveBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	fmt.Println("Waiting for router to apply (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished.")
}
