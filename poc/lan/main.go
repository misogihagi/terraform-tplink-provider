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
		Headless: playwright.Bool(false), // Set to false to see the interaction for PoC
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

	// 2. Browser Context with basic auth
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		HttpCredentials: &playwright.HttpCredentials{
			Username: "admin",
			Password: "admin",
		},
	})
	if err != nil {
		log.Fatalf("could not create context: %v", err)
	}

	page, err := context.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}

	endpoint := "http://192.168.1.1"
	newIP := "192.168.11.1" // Set current or new IP

	fmt.Printf("Navigating to %s...\n", endpoint)
	if _, err = page.Goto(endpoint); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	// 3. Handle login (if not handled by basic auth)
	// Some models use a form even if basic auth is present
	fmt.Println("Waiting for login elements...")
	_ = page.Locator("#userName").Fill("admin")
	_ = page.Locator("#pcPassword").Fill("admin")
	_ = page.Locator("#loginBtn").Click()

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	// 4. Extract frames and navigate
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

	fmt.Println("Clicking Network menu...")
	networkLoc := leftFrame.Locator("a:has-text('ネットワーク'), a:has-text('Network'), #a2").First()
	if err := networkLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for network link: %v", err)
	}
	if err := networkLoc.Click(); err != nil {
		log.Fatalf("could not click network menu: %v", err)
	}

	time.Sleep(1 * time.Second)

	fmt.Println("Clicking LAN submenu...")
	lanLoc := leftFrame.Locator("a:has-text('LAN'), #a4").First()
	if err := lanLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for lan link: %v", err)
	}
	if err := lanLoc.Click(); err != nil {
		log.Fatalf("could not click lan submenu: %v", err)
	}

	// 5. Update LAN IP in mainFrame
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

	fmt.Printf("Filling new IP: %s\n", newIP)
	ipInput := mainFrame.Locator("input#lanIp, input[name='ip']").First()
	if err := ipInput.WaitFor(); err != nil {
		log.Fatalf("could not wait for IP address input: %v", err)
	}

	if err := ipInput.Fill(newIP); err != nil {
		log.Fatalf("could not fill IP address: %v", err)
	}

	// 6. Handle Save and Reboot
	fmt.Println("Clicking Save button...")
	saveBtn := mainFrame.Locator("input#saveBtn, input.buttonBig[value*='保存'], input.buttonBig[value*='Save']").First()

	// Capture dialog for reboot confirmation
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		dialog.Accept()
	})

	if err := saveBtn.Click(); err != nil {
		log.Fatalf("could not click save: %v", err)
	}

	fmt.Println("Waiting for router to process request (5s)...")
	time.Sleep(5 * time.Second)

	fmt.Println("PoC Finished")
}
