// blocker.go
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	reset  = "\033[0m"
	green  = "\033[92m"
	red    = "\033[91m"
	yellow = "\033[93m"
	blue   = "\033[94m"
)

func colorize(text, color string) string {
	return color + text + reset
}

type Config struct {
	Blocked   []string `json:"blocked"`
	Active    bool     `json:"active"`
	TimerEnd  string   `json:"timer_end,omitempty"`
	Whitelist []string `json:"whitelist"`
}

func getConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".blocker")
}

func getConfigFile() string {
	return filepath.Join(getConfigDir(), "config.json")
}

func getHostsFile() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\System32\drivers\etc\hosts`
	}
	return "/etc/hosts"
}

const blockIP = "127.0.0.1"

func ensureConfig() {
	os.MkdirAll(getConfigDir(), 0755)
	if _, err := os.Stat(getConfigFile()); os.IsNotExist(err) {
		cfg := Config{Blocked: []string{}, Active: false, Whitelist: []string{}}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		ioutil.WriteFile(getConfigFile(), data, 0644)
	}
}

func loadConfig() Config {
	ensureConfig()
	data, err := ioutil.ReadFile(getConfigFile())
	if err != nil {
		return Config{}
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	ioutil.WriteFile(getConfigFile(), data, 0644)
}

func readHosts() ([]string, error) {
	file, err := os.Open(getHostsFile())
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeHosts(lines []string) error {
	return ioutil.WriteFile(getHostsFile(), []byte(strings.Join(lines, "\n")), 0644)
}

func updateHosts(domains []string, action string) error {
	lines, err := readHosts()
	if err != nil {
		return err
	}
	var newLines []string
	for _, line := range lines {
		skip := false
		for _, domain := range domains {
			if strings.HasPrefix(strings.TrimSpace(line), blockIP) && strings.Contains(line, domain) {
				skip = true
				break
			}
		}
		if !skip {
			newLines = append(newLines, line)
		}
	}
	if action == "add" {
		for _, domain := range domains {
			newLines = append(newLines, fmt.Sprintf("%s %s", blockIP, domain))
		}
	}
	return writeHosts(newLines)
}

func startBlocking(domains []string, durationMinutes int) {
	cfg := loadConfig()
	cfg.Blocked = append(cfg.Blocked, domains...)
	if durationMinutes > 0 {
		cfg.TimerEnd = time.Now().Add(time.Duration(durationMinutes) * time.Minute).Format(time.RFC3339)
	}
	cfg.Active = true
	saveConfig(cfg)

	if err := updateHosts(domains, "add"); err != nil {
		fmt.Println(colorize("Error: "+err.Error(), red))
		os.Exit(1)
	}
	fmt.Println(colorize(fmt.Sprintf("Blocked %d domains.", len(domains)), green))
	if durationMinutes > 0 {
		fmt.Println(colorize(fmt.Sprintf("Blocking active for %d min.", durationMinutes), yellow))
		go timerThread(durationMinutes)
	}
}

func timerThread(minutes int) {
	time.Sleep(time.Duration(minutes) * time.Minute)
	fmt.Println(colorize("Time expired. Unblocking...", yellow))
	disableBlocking()
}

func disableBlocking() {
	cfg := loadConfig()
	blocked := cfg.Blocked
	if len(blocked) == 0 {
		fmt.Println(colorize("No active blocking.", yellow))
		return
	}
	if err := updateHosts(blocked, "remove"); err != nil {
		fmt.Println(colorize("Error: "+err.Error(), red))
		return
	}
	cfg.Blocked = []string{}
	cfg.Active = false
	cfg.TimerEnd = ""
	saveConfig(cfg)
	fmt.Println(colorize("Blocking disabled.", green))
}

func showStatus() {
	cfg := loadConfig()
	if cfg.Active && cfg.TimerEnd != "" {
		end, _ := time.Parse(time.RFC3339, cfg.TimerEnd)
		remaining := int(end.Sub(time.Now()).Seconds())
		if remaining > 0 {
			mins := remaining / 60
			secs := remaining % 60
			fmt.Println(colorize(fmt.Sprintf("Blocking active. Remaining: %d min %d sec", mins, secs), blue))
		} else {
			fmt.Println(colorize("Blocking active but timer expired.", yellow))
		}
	} else if cfg.Active {
		fmt.Println(colorize("Blocking active (no timer).", blue))
	} else {
		fmt.Println(colorize("Blocking inactive.", yellow))
	}
	if len(cfg.Blocked) > 0 {
		fmt.Println(colorize(fmt.Sprintf("Blocked domains (%d):", len(cfg.Blocked)), green))
		for _, d := range cfg.Blocked {
			fmt.Printf("  - %s\n", d)
		}
	} else {
		fmt.Println(colorize("No blocked domains.", yellow))
	}
}

func main() {
	var (
		addCmd      = flag.NewFlagSet("add", flag.ExitOnError)
		removeCmd   = flag.NewFlagSet("remove", flag.ExitOnError)
		listCmd     = flag.NewFlagSet("list", flag.ExitOnError)
		enableCmd   = flag.NewFlagSet("enable", flag.ExitOnError)
		disableCmd  = flag.NewFlagSet("disable", flag.ExitOnError)
		statusCmd   = flag.NewFlagSet("status", flag.ExitOnError)
		pomodoroCmd = flag.NewFlagSet("pomodoro", flag.ExitOnError)
		clearCmd    = flag.NewFlagSet("clear", flag.ExitOnError)
	)

	if len(os.Args) < 2 {
		fmt.Println(colorize("Usage: blocker <add|remove|list|enable|disable|status|pomodoro|clear> [domains...] [options]", yellow))
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		addCmd.Parse(os.Args[2:])
		domains := addCmd.Args()
		if len(domains) == 0 {
			fmt.Println(colorize("Specify domains to block.", yellow))
			return
		}
		startBlocking(domains, 0)

	case "remove":
		removeCmd.Parse(os.Args[2:])
		domains := removeCmd.Args()
		if len(domains) == 0 {
			fmt.Println(colorize("Specify domains to remove.", yellow))
			return
		}
		cfg := loadConfig()
		for _, d := range domains {
			for i, b := range cfg.Blocked {
				if b == d {
					cfg.Blocked = append(cfg.Blocked[:i], cfg.Blocked[i+1:]...)
					break
				}
			}
		}
		saveConfig(cfg)
		if err := updateHosts(domains, "remove"); err != nil {
			fmt.Println(colorize("Error: "+err.Error(), red))
		} else {
			fmt.Println(colorize(fmt.Sprintf("Removed: %s", strings.Join(domains, ", ")), green))
		}

	case "list":
		listCmd.Parse(os.Args[2:])
		cfg := loadConfig()
		blocked := cfg.Blocked
		if len(blocked) == 0 {
			fmt.Println(colorize("No blocked domains.", yellow))
		} else {
			fmt.Println(colorize("Blocked domains:", green))
			for _, d := range blocked {
				fmt.Printf("  - %s\n", d)
			}
		}

	case "enable":
		timeFlag := enableCmd.Int("t", 0, "Duration in minutes")
		enableCmd.Parse(os.Args[2:])
		cfg := loadConfig()
		if len(cfg.Blocked) == 0 {
			fmt.Println(colorize("No domains in blocklist. Add them with 'add'.", yellow))
			return
		}
		startBlocking(cfg.Blocked, *timeFlag)

	case "disable":
		disableCmd.Parse(os.Args[2:])
		disableBlocking()

	case "status":
		statusCmd.Parse(os.Args[2:])
		showStatus()

	case "pomodoro":
		pomodoroCmd.Parse(os.Args[2:])
		cfg := loadConfig()
		if len(cfg.Blocked) == 0 {
			fmt.Println(colorize("No domains in blocklist. Add them with 'add'.", yellow))
			return
		}
		fmt.Println(colorize("Starting Pomodoro: 25 minutes work", blue))
		startBlocking(cfg.Blocked, 25)
		time.Sleep(25 * time.Minute)
		fmt.Println(colorize("Pomodoro finished! 5 minutes break.", green))
		disableBlocking()
		time.Sleep(5 * time.Minute)
		fmt.Println(colorize("Break over. Start next Pomodoro.", blue))

	case "clear":
		force := clearCmd.Bool("f", false, "Force")
		clearCmd.Parse(os.Args[2:])
		if !*force {
			fmt.Print(colorize("Clear all blocklist? [y/N] ", yellow))
			var ans string
			fmt.Scanln(&ans)
			if ans != "y" && ans != "Y" {
				return
			}
		}
		cfg := loadConfig()
		if len(cfg.Blocked) == 0 {
			fmt.Println(colorize("Blocklist already empty.", yellow))
			return
		}
		if err := updateHosts(cfg.Blocked, "remove"); err != nil {
			fmt.Println(colorize("Error: "+err.Error(), red))
		} else {
			cfg.Blocked = []string{}
			cfg.Active = false
			saveConfig(cfg)
			fmt.Println(colorize("Blocklist cleared.", green))
		}

	default:
		fmt.Println(colorize("Unknown command: "+os.Args[1], red))
	}
}
