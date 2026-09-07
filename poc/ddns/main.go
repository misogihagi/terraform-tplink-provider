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

	// DDNS Settings
	provider := "noipDns"      // dynDns | noipDns | cmxDns
	dynDomain := ""            // dyn.com/dns domain
	noipDomain := "example.ddns.net" // noip domain
	cmxDomains := []string{}   // comexe domains 1-5
	ddnsUser := "my-dyndns-user"
	ddnsPass := "my-dyndns-pass"
	wanIPBinding := true       // true: enable, false: disable
	ddnsEnable := true
	doLogin := true            // click the ログイン(login) button to test credentials

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

	// 3. Navigate to DDNS menu
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

	fmt.Println("Clicking DDNS menu...")
	ddnsMenuLoc := leftFrame.Locator("a:has-text('DDNS'), a:has-text('DynDNS')").First()
	if err := ddnsMenuLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for DDNS menu: %v", err)
	}
	_ = ddnsMenuLoc.Click()
	time.Sleep(1 * time.Second)

	fmt.Println("Clicking DDNS Settings submenu...")
	ddnsSettingsMenuLoc := leftFrame.Locator("a:has-text('DDNS 設定'), a:has-text('DDNS Settings')").First()
	if err := ddnsSettingsMenuLoc.WaitFor(); err == nil {
		_ = ddnsSettingsMenuLoc.Click()
		time.Sleep(1 * time.Second)
	}

	// 4. entering settings (mainFrame)
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

	// Wait for the page to render by checking the provider select
	if err := mainFrame.Locator("select#ddns_server").WaitFor(); err != nil {
		log.Fatalf("could not wait for DDNS page to load: %v", err)
	}

	// Select Service Provider
	fmt.Printf("Selecting provider: %s\n", provider)
	_, _ = mainFrame.Locator("select#ddns_server").SelectOption(playwright.SelectOptionValues{
		Values: playwright.StringSlice(provider),
	})
	time.Sleep(1 * time.Second)

	// Fill Domain Name(s) - only the visible one for the selected provider is filled
	switch provider {
	case "dynDns":
		if dynDomain != "" {
			fmt.Printf("Setting DynDNS domain: %s\n", dynDomain)
			_ = mainFrame.Locator("input#dynDomain").Fill(dynDomain)
		}
	case "noipDns":
		if noipDomain != "" {
			fmt.Printf("Setting No-IP domain: %s\n", noipDomain)
			_ = mainFrame.Locator("input#noipDomain").Fill(noipDomain)
		}
	case "cmxDns":
		for i, d := range cmxDomains {
			fmt.Printf("Setting Comexe domain %d: %s\n", i+1, d)
			_ = mainFrame.Locator(fmt.Sprintf("input#cmxDomain%d", i+1)).Fill(d)
		}
	}

	// Username & Password
	fmt.Printf("Setting username: %s\n", ddnsUser)
	_ = mainFrame.Locator("input#ddns_usr").Fill(ddnsUser)
	fmt.Println("Setting password...")
	_ = mainFrame.Locator("input#ddns_pwd").Fill(ddnsPass)

	// WAN IP Binding
	if wanIPBinding {
		fmt.Println("Enabling WAN IP Binding...")
		_ = mainFrame.Locator("input#WanIPBindingEnable").Check()
	} else {
		fmt.Println("Disabling WAN IP Binding...")
		_ = mainFrame.Locator("input#WanIPBindingDisable").Check()
	}

	// DDNS Enable checkbox
	if ddnsEnable {
		fmt.Println("Enabling DDNS...")
		_ = mainFrame.Locator("input#ddns_enable").Check()
	} else {
		fmt.Println("Disabling DDNS...")
		_ = mainFrame.Locator("input#ddns_enable").Uncheck()
	}

	// 5. Login to the DDNS provider (tests credentials), then Save
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog: %s\n", dialog.Message())
		dialog.Accept()
	})

	if doLogin {
		fmt.Println("Clicking Login button...")
		loginBtn := mainFrame.Locator("input#login").First()
		if err := loginBtn.WaitFor(); err != nil {
			log.Fatalf("could not wait for login button: %v", err)
		}
		_ = loginBtn.Click()
		time.Sleep(5 * time.Second)
	}

	fmt.Println("Saving settings...")
	saveBtn := mainFrame.Locator("input#save").First()
	if err := saveBtn.WaitFor(); err != nil {
		log.Fatalf("could not wait for save button: %v", err)
	}
	_ = saveBtn.Click()

	fmt.Println("Waiting for router to apply (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished.")
}
