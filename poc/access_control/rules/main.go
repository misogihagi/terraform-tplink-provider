package main

import (
	"fmt"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
)

// Config holds the settings to apply to the Access Control Rules (アクセス制御ルール) page.
type Config struct {
	// Enabled enables or disables Internet Access Control.
	Enabled bool

	// DefaultAction determines the default filtering rule.
	//   true  -> Allow (許可): packets not specified by filtering rules are denied.
	//   false -> Deny (拒否): packets not specified by filtering rules are denied.
	DefaultAction bool

	// Rules is the list of access control rules to add.
	Rules []Rule
}

// Rule represents a single access control rule entry.
type Rule struct {
	// Description is a human-readable name for the rule.
	Description string

	// LANHost is the LAN host (e.g. MAC address or IP) this rule applies to.
	// Use "*" for "All".
	LANHost string

	// Target is the target (e.g. URL or IP range) this rule applies to.
	// Use "*" for "All".
	Target string

	// Schedule is the schedule name this rule uses.
	// Use "" for "Always".
	Schedule string

	// Allow determines the rule action: true = Allow, false = Deny.
	Allow bool
}

func main() {
	// --- Target configuration ---
	cfg := Config{
		Enabled:       true,
		DefaultAction: false, // Deny (拒否)
		Rules: []Rule{
			{
				Description: "Allow PC1",
				LANHost:     "AA:BB:CC:DD:EE:FF",
				Target:      "*",
				Schedule:    "",
				Allow:       true,
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
		Headless: playwright.Bool(false), // Set to false to see the interaction for PoC
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

	// 4. Find bottomLeftFrame and navigate to Access Control > Rules
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

	fmt.Println("Clicking Rules submenu...")
	rulesLoc := leftFrame.Locator("a:has-text('ルール'), a:has-text('Rules')").First()
	if err := rulesLoc.WaitFor(); err != nil {
		log.Fatalf("could not wait for Rules link: %v", err)
	}
	if err := rulesLoc.Click(); err != nil {
		log.Fatalf("could not click Rules submenu: %v", err)
	}

	// 5. Find mainFrame and interact with the controls
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

	fmt.Println("Waiting for Access Control enable checkbox...")
	enableChk := mainFrame.Locator("input#enableFw")
	if err := enableChk.WaitFor(); err != nil {
		log.Fatalf("could not wait for enable checkbox: %v", err)
	}

	// 6. Enable/disable Internet Access Control
	fmt.Printf("Setting Internet Access Control enabled=%v...\n", cfg.Enabled)
	if err := enableChk.SetChecked(cfg.Enabled); err != nil {
		log.Fatalf("could not set enable checkbox: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 7. Set default filtering rule
	fmt.Printf("Setting default filtering rule (allow=%v)...\n", cfg.DefaultAction)
	if cfg.DefaultAction {
		if err := mainFrame.Locator("input#act_en").Check(); err != nil {
			log.Fatalf("could not check Allow radio: %v", err)
		}
	} else {
		if err := mainFrame.Locator("input#act_dis").Check(); err != nil {
			log.Fatalf("could not check Deny radio: %v", err)
		}
	}

	// 8. Add rules
	for i, rule := range cfg.Rules {
		fmt.Printf("Adding rule %d: %s...\n", i+1, rule.Description)

		// Click "Add New" button
		addBtn := mainFrame.Locator("input.T_addnew").First()
		if err := addBtn.Click(); err != nil {
			log.Fatalf("could not click Add New button: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Fill in Description
		if rule.Description != "" {
			descInput := mainFrame.Locator("input#ruleName, input[name='ruleName']").First()
			if err := descInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for description input: %v", err)
			}
			if err := descInput.Fill(rule.Description); err != nil {
				log.Fatalf("could not fill description: %v", err)
			}
		}

		// Set LAN Host (MAC address)
		if rule.LANHost != "" && rule.LANHost != "*" {
			macInput := mainFrame.Locator("input#mac, input[name='mac']").First()
			if err := macInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for MAC input: %v", err)
			}
			if err := macInput.Fill(rule.LANHost); err != nil {
				log.Fatalf("could not fill MAC: %v", err)
			}
		}

		// Set Target
		if rule.Target != "" && rule.Target != "*" {
			targetInput := mainFrame.Locator("input#target, input[name='target']").First()
			if err := targetInput.WaitFor(); err != nil {
				log.Fatalf("could not wait for target input: %v", err)
			}
			if err := targetInput.Fill(rule.Target); err != nil {
				log.Fatalf("could not fill target: %v", err)
			}
		}

		// Set Schedule
		if rule.Schedule != "" {
			scheduleSelect := mainFrame.Locator("select#schedId, select[name='schedId']").First()
			if err := scheduleSelect.WaitFor(); err != nil {
				log.Fatalf("could not wait for schedule select: %v", err)
			}
			if _, err := scheduleSelect.SelectOption(playwright.SelectOptionValues{
				Labels: &[]string{rule.Schedule},
			}); err != nil {
				log.Fatalf("could not select schedule: %v", err)
			}
		}

		// Set Rule action (Allow/Deny)
		ruleSelect := mainFrame.Locator("select#ruleId, select[name='ruleId']").First()
		if err := ruleSelect.WaitFor(); err != nil {
			log.Fatalf("could not wait for rule select: %v", err)
		}
		if rule.Allow {
			if _, err := ruleSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"1"},
			}); err != nil {
				log.Fatalf("could not select Allow rule: %v", err)
			}
		} else {
			if _, err := ruleSelect.SelectOption(playwright.SelectOptionValues{
				Values: &[]string{"0"},
			}); err != nil {
				log.Fatalf("could not select Deny rule: %v", err)
			}
		}

		// Save the rule
		fmt.Printf("Saving rule %d...\n", i+1)
		saveRuleBtn := mainFrame.Locator("input[onclick*='doSave'], input#saveBtn, input[value='保存'], input[value='Save']").First()
		if err := saveRuleBtn.Click(); err != nil {
			log.Fatalf("could not click save rule button: %v", err)
		}
		time.Sleep(1 * time.Second)

		// Handle any confirmation dialog
		page.OnDialog(func(dialog playwright.Dialog) {
			fmt.Printf("Dialog appeared: %s\n", dialog.Message())
			dialog.Accept()
		})
	}

	// 9. Save the main settings
	fmt.Println("Clicking main Save button...")
	saveBtn := mainFrame.Locator("input.button.L.T.T_save, input#saveBtnClk, input[onclick='doClkSave();']").First()

	// Capture any confirmation dialogs
	page.OnDialog(func(dialog playwright.Dialog) {
		fmt.Printf("Dialog appeared: %s\n", dialog.Message())
		dialog.Accept()
	})

	if err := saveBtn.Click(); err != nil {
		log.Fatalf("could not click save: %v", err)
	}

	fmt.Println("Waiting for router to process request (3s)...")
	time.Sleep(3 * time.Second)

	fmt.Println("PoC Finished")
}
