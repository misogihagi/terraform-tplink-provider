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
		Headless: playwright.Bool(false), // 目視確認のため false
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

	// セキュリティ設定
	pskPassword := "testpassword123"

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

	// 5. 「ワイヤレス セキュリティ」サブメニューをクリック
	fmt.Println("Clicking ワイヤレス セキュリティ submenu...")
	securitySettingsLoc := leftFrame.Locator("a:has-text('ワイヤレス セキュリティ'), a:has-text('Wireless Security')").First()
	if err := securitySettingsLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for ワイヤレス セキュリティ link: %v", err)
	}
	if err := securitySettingsLoc.Click(); err != nil {
		log.Fatalf("could not click ワイヤレス セキュリティ submenu: %v", err)
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

	// 7. セキュリティ設定の構成
	fmt.Println("Configuring Wireless Security settings...")

	// WPA/WPA2 - Personal を選択
	secPSKRadio := mainFrame.Locator("input#secPSK")
	if err := secPSKRadio.WaitFor(); err != nil {
		log.Fatalf("could not wait for secPSK radio: %v", err)
	}
	if err := secPSKRadio.Check(); err != nil {
		log.Fatalf("could not check secPSK radio: %v", err)
	}

	// バージョン: WPA2-PSK (value="2")
	pskAuthTypeSelect := mainFrame.Locator("select#pskAuthType")
	if _, err := pskAuthTypeSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}}); err != nil {
		log.Fatalf("could not select pskAuthType: %v", err)
	}

	// 暗号化: AES (value="2")
	pskCipherSelect := mainFrame.Locator("select#pskCipher")
	if _, err := pskCipherSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{"2"}}); err != nil {
		log.Fatalf("could not select pskCipher: %v", err)
	}

	// ワイヤレス パスワード
	pskSecretInput := mainFrame.Locator("input#pskSecret")
	if err := pskSecretInput.Fill(pskPassword); err != nil {
		log.Fatalf("could not fill pskSecret: %v", err)
	}

	// 8. 保存
	fmt.Println("Saving settings...")
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})
	saveBtn := mainFrame.Locator("input.T_save, input#save, input[onclick*='doSave']").First()
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
