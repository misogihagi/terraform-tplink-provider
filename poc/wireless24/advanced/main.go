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

	// 詳細設定の値
	txPower := "100"      // 100=高, 50=中, 20=低
	beaconInt := "100"    // 40-1000
	rtsInt := "2346"      // 1-2346
	fragTh := "2346"      // 256-2346
	dtimTh := "1"         // 1-15
	shortGI := true
	clientIso := false
	wmm := true // WMM (DOMでは disabled だったが、通常は有効可能)

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

	// 5. 「ワイヤレス詳細設定」サブメニューをクリック
	fmt.Println("Clicking ワイヤレス詳細設定 submenu...")
	advancedSettingsLoc := leftFrame.Locator("a:has-text('ワイヤレス詳細設定'), a:has-text('Wireless Advanced')").First()
	if err := advancedSettingsLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for ワイヤレス詳細設定 link: %v", err)
	}
	if err := advancedSettingsLoc.Click(); err != nil {
		log.Fatalf("could not click ワイヤレス詳細設定 submenu: %v", err)
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

	// 7. 各設定項目への入力
	fmt.Println("Configuring Advanced Settings...")

	// 転送電力
	fmt.Printf("Setting Transfer Power: %s\n", txPower)
	txPowerSelect := mainFrame.Locator("select#txPower").First()
	if _, err := txPowerSelect.SelectOption(playwright.SelectOptionValues{Values: &[]string{txPower}}); err != nil {
		log.Printf("could not select txPower: %v", err)
	}

	// ビーコン間隔
	fmt.Printf("Setting Beacon Interval: %s\n", beaconInt)
	beaconInput := mainFrame.Locator("input#beaconInt").First()
	if err := beaconInput.Fill(beaconInt); err != nil {
		log.Printf("could not fill beaconInt: %v", err)
	}

	// RTS しきい値
	fmt.Printf("Setting RTS Threshold: %s\n", rtsInt)
	rtsInput := mainFrame.Locator("input#rtsInt").First()
	if err := rtsInput.Fill(rtsInt); err != nil {
		log.Printf("could not fill rtsInt: %v", err)
	}

	// 断片化しきい値
	fmt.Printf("Setting Fragmentation Threshold: %s\n", fragTh)
	fragInput := mainFrame.Locator("input#fragTh").First()
	if err := fragInput.Fill(fragTh); err != nil {
		log.Printf("could not fill fragTh: %v", err)
	}

	// DTIM 間隔
	fmt.Printf("Setting DTIM Interval: %s\n", dtimTh)
	dtimInput := mainFrame.Locator("input#dtimTh").First()
	if err := dtimInput.Fill(dtimTh); err != nil {
		log.Printf("could not fill dtimTh: %v", err)
	}

	// ショート GI
	shortGICheckbox := mainFrame.Locator("input#shortGI").First()
	if shortGI {
		fmt.Println("Enabling Short GI...")
		_ = shortGICheckbox.Check()
	} else {
		fmt.Println("Disabling Short GI...")
		_ = shortGICheckbox.Uncheck()
	}

	// クライアント切り離し
	clientIsoCheckbox := mainFrame.Locator("input#clientIso").First()
	if clientIso {
		fmt.Println("Enabling Client Isolation...")
		_ = clientIsoCheckbox.Check()
	} else {
		fmt.Println("Disabling Client Isolation...")
		_ = clientIsoCheckbox.Uncheck()
	}

	// WMM
	wmmCheckbox := mainFrame.Locator("input#wmeEn").First()
	isDisabled, _ := wmmCheckbox.IsDisabled()
	if isDisabled {
		fmt.Println("WMM checkbox is disabled, skipping...")
	} else {
		if wmm {
			fmt.Println("Enabling WMM...")
			_ = wmmCheckbox.Check()
		} else {
			fmt.Println("Disabling WMM...")
			_ = wmmCheckbox.Uncheck()
		}
	}

	// 8. 保存
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
