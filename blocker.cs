// blocker.cs
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Runtime.InteropServices;

class Blocker
{
    static string Colorize(string text, string color)
    {
        string col = color switch
        {
            "green" => "\x1b[92m",
            "red" => "\x1b[91m",
            "yellow" => "\x1b[93m",
            "blue" => "\x1b[94m",
            _ => "\x1b[0m"
        };
        return col + text + "\x1b[0m";
    }

    class Config
    {
        public List<string> Blocked { get; set; } = new();
        public bool Active { get; set; }
        public string TimerEnd { get; set; }
        public List<string> Whitelist { get; set; } = new();
    }

    static string ConfigDir => Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.UserProfile), ".blocker");
    static string ConfigFile => Path.Combine(ConfigDir, "config.json");
    static string HostsFile => RuntimeInformation.IsOSPlatform(OSPlatform.Windows)
        ? @"C:\Windows\System32\drivers\etc\hosts"
        : "/etc/hosts";
    const string BLOCK_IP = "127.0.0.1";

    static void EnsureConfig()
    {
        Directory.CreateDirectory(ConfigDir);
        if (!File.Exists(ConfigFile))
        {
            var def = new Config();
            File.WriteAllText(ConfigFile, JsonSerializer.Serialize(def, new JsonSerializerOptions { WriteIndented = true }));
        }
    }

    static Config LoadConfig()
    {
        EnsureConfig();
        var json = File.ReadAllText(ConfigFile);
        return JsonSerializer.Deserialize<Config>(json) ?? new Config();
    }

    static void SaveConfig(Config cfg)
    {
        var json = JsonSerializer.Serialize(cfg, new JsonSerializerOptions { WriteIndented = true });
        File.WriteAllText(ConfigFile, json);
    }

    static List<string> ReadHosts()
    {
        return File.ReadAllLines(HostsFile).ToList();
    }

    static void WriteHosts(List<string> lines)
    {
        File.WriteAllLines(HostsFile, lines);
    }

    static void UpdateHosts(List<string> domains, string action)
    {
        var lines = ReadHosts();
        var newLines = lines.Where(line =>
        {
            foreach (var d in domains)
                if (line.Trim().StartsWith(BLOCK_IP) && line.Contains(d))
                    return false;
            return true;
        }).ToList();
        if (action == "add")
        {
            foreach (var d in domains)
                newLines.Add($"{BLOCK_IP} {d}");
        }
        WriteHosts(newLines);
    }

    static void StartBlocking(List<string> domains, int minutes)
    {
        var cfg = LoadConfig();
        cfg.Blocked = cfg.Blocked.Union(domains).ToList();
        if (minutes > 0)
            cfg.TimerEnd = DateTime.Now.AddMinutes(minutes).ToString("o");
        cfg.Active = true;
        SaveConfig(cfg);
        try
        {
            UpdateHosts(domains, "add");
            Console.WriteLine(Colorize($"Blocked {domains.Count} domains.", "green"));
            if (minutes > 0)
            {
                Console.WriteLine(Colorize($"Blocking active for {minutes} min.", "yellow"));
                new Thread(() => TimerThread(minutes)).Start();
            }
        }
        catch (Exception e)
        {
            Console.WriteLine(Colorize($"Error: {e.Message}", "red"));
            Environment.Exit(1);
        }
    }

    static void TimerThread(int minutes)
    {
        Thread.Sleep(minutes * 60000);
        Console.WriteLine(Colorize("Time expired. Unblocking...", "yellow"));
        DisableBlocking();
    }

    static void DisableBlocking()
    {
        var cfg = LoadConfig();
        if (cfg.Blocked.Count == 0)
        {
            Console.WriteLine(Colorize("No active blocking.", "yellow"));
            return;
        }
        try
        {
            UpdateHosts(cfg.Blocked, "remove");
            cfg.Blocked.Clear();
            cfg.Active = false;
            cfg.TimerEnd = null;
            SaveConfig(cfg);
            Console.WriteLine(Colorize("Blocking disabled.", "green"));
        }
        catch (Exception e)
        {
            Console.WriteLine(Colorize($"Error: {e.Message}", "red"));
        }
    }

    static void ShowStatus()
    {
        var cfg = LoadConfig();
        if (cfg.Active && !string.IsNullOrEmpty(cfg.TimerEnd))
        {
            var end = DateTime.Parse(cfg.TimerEnd);
            var remaining = (end - DateTime.Now).TotalSeconds;
            if (remaining > 0)
            {
                var mins = (int)remaining / 60;
                var secs = (int)remaining % 60;
                Console.WriteLine(Colorize($"Blocking active. Remaining: {mins} min {secs} sec", "blue"));
            }
            else
                Console.WriteLine(Colorize("Blocking active but timer expired.", "yellow"));
        }
        else if (cfg.Active)
            Console.WriteLine(Colorize("Blocking active (no timer).", "blue"));
        else
            Console.WriteLine(Colorize("Blocking inactive.", "yellow"));
        if (cfg.Blocked.Count > 0)
        {
            Console.WriteLine(Colorize($"Blocked domains ({cfg.Blocked.Count}):", "green"));
            foreach (var d in cfg.Blocked) Console.WriteLine($"  - {d}");
        }
        else
            Console.WriteLine(Colorize("No blocked domains.", "yellow"));
    }

    static void Main(string[] args)
    {
        if (args.Length < 1)
        {
            Console.WriteLine(Colorize("Usage: blocker add|remove|list|enable|disable|status|pomodoro|clear [domains...] [options]", "yellow"));
            return;
        }
        var cmd = args[0];
        var rest = args.Skip(1).ToList();

        EnsureConfig();
        var cfg = LoadConfig();

        switch (cmd)
        {
            case "add":
                if (rest.Count == 0)
                {
                    Console.WriteLine(Colorize("Specify domains to block.", "yellow"));
                    return;
                }
                var idx = rest.IndexOf("-t");
                int minutes = 0;
                if (idx != -1 && rest.Count > idx+1)
                {
                    minutes = int.Parse(rest[idx+1]);
                    rest.RemoveRange(idx, 2);
                }
                StartBlocking(rest, minutes);
                break;

            case "remove":
                if (rest.Count == 0)
                {
                    Console.WriteLine(Colorize("Specify domains to remove.", "yellow"));
                    return;
                }
                foreach (var d in rest) cfg.Blocked.Remove(d);
                SaveConfig(cfg);
                try
                {
                    UpdateHosts(rest, "remove");
                    Console.WriteLine(Colorize($"Removed: {string.Join(", ", rest)}", "green"));
                }
                catch (Exception e)
                {
                    Console.WriteLine(Colorize($"Error: {e.Message}", "red"));
                }
                break;

            case "list":
                if (cfg.Blocked.Count == 0)
                    Console.WriteLine(Colorize("No blocked domains.", "yellow"));
                else
                {
                    Console.WriteLine(Colorize("Blocked domains:", "green"));
                    foreach (var d in cfg.Blocked) Console.WriteLine($"  - {d}");
                }
                break;

            case "enable":
                if (cfg.Blocked.Count == 0)
                {
                    Console.WriteLine(Colorize("No domains in blocklist. Add them with 'add'.", "yellow"));
                    return;
                }
                var timeIdx = rest.IndexOf("-t");
                int dur = 0;
                if (timeIdx != -1 && rest.Count > timeIdx+1)
                    dur = int.Parse(rest[timeIdx+1]);
                StartBlocking(cfg.Blocked, dur);
                break;

            case "disable":
                DisableBlocking();
                break;

            case "status":
                ShowStatus();
                break;

            case "pomodoro":
                if (cfg.Blocked.Count == 0)
                {
                    Console.WriteLine(Colorize("No domains in blocklist. Add them with 'add'.", "yellow"));
                    return;
                }
                Console.WriteLine(Colorize("Starting Pomodoro: 25 minutes work", "blue"));
                StartBlocking(cfg.Blocked, 25);
                Thread.Sleep(25 * 60000);
                Console.WriteLine(Colorize("Pomodoro finished! 5 minutes break.", "green"));
                DisableBlocking();
                Thread.Sleep(5 * 60000);
                Console.WriteLine(Colorize("Break over. Start next Pomodoro.", "blue"));
                break;

            case "clear":
                bool force = rest.Contains("-f");
                if (!force)
                {
                    Console.Write(Colorize("Clear all blocklist? [y/N] ", "yellow"));
                    var ans = Console.ReadLine();
                    if (ans?.ToLower() != "y") return;
                }
                if (cfg.Blocked.Count == 0)
                {
                    Console.WriteLine(Colorize("Blocklist already empty.", "yellow"));
                    return;
                }
                try
                {
                    UpdateHosts(cfg.Blocked, "remove");
                    cfg.Blocked.Clear();
                    cfg.Active = false;
                    cfg.TimerEnd = null;
                    SaveConfig(cfg);
                    Console.WriteLine(Colorize("Blocklist cleared.", "green"));
                }
                catch (Exception e)
                {
                    Console.WriteLine(Colorize($"Error: {e.Message}", "red"));
                }
                break;

            default:
                Console.WriteLine(Colorize($"Unknown command: {cmd}", "red"));
                break;
        }
    }
}
