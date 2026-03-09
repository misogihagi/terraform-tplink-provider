package main

import (
	"fmt"
	"log"

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
		Headless: playwright.Bool(true),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	defer browser.Close()

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

	fmt.Println("Navigating to 192.168.1.1...")
	if _, err = page.Goto("http://192.168.1.1"); err != nil {
		log.Fatalf("could not goto: %v", err)
	}

	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		log.Printf("wait for load state error: %v", err)
	}

	fmt.Println("Extracting frames...")
	frames := page.Frames()
	for _, f := range frames {
		fmt.Printf("Frame: %s (URL: %s)\n", f.Name(), f.URL())
	}

	fmt.Println("Done")
}
