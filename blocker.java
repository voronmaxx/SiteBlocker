// blocker.java
import java.io.*;
import java.nio.file.*;
import java.time.*;
import java.time.format.*;
import java.util.*;
import java.util.stream.*;
import com.google.gson.*;

public class blocker {
    private static final String RESET = "\u001B[0m";
    private static final String GREEN = "\u001B[92m";
    private static final String RED = "\u001B[91m";
    private static final String YELLOW = "\u001B[93m";
    private static final String BLUE = "\u001B[94m";

    private static String colorize(String text, String color) {
        return color + text + RESET;
    }

    private static class Config {
        List<String> blocked = new ArrayList<>();
        boolean active;
        String timerEnd;
        List<String> whitelist = new ArrayList<>();
    }

    private static String configDir = System.getProperty("user.home") + "/.blocker";
    private static String configFile = configDir + "/config.json";
    private static String hostsFile = System.getProperty("os.name").toLowerCase().contains("win")
        ? "C:\\Windows\\System32\\drivers\\etc\\hosts"
        : "/etc/hosts";
    private static final String BLOCK_IP = "127.0.0.1";

    private static void ensureConfig() throws IOException {
        Files.createDirectories(Paths.get(configDir));
        if (!Files.exists(Paths.get(configFile))) {
            Config def = new Config();
            saveConfig(def);
        }
    }

    private static Config loadConfig() throws IOException {
        ensureConfig();
        String json = new String(Files.readAllBytes(Paths.get(configFile)));
        Gson gson = new Gson();
        return gson.fromJson(json, Config.class);
    }

    private static void saveConfig(Config cfg) throws IOException {
        Gson gson = new GsonBuilder().setPrettyPrinting().create();
        String json = gson.toJson(cfg);
        Files.write(Paths.get(configFile), json.getBytes());
    }

    private static List<String> readHosts() throws IOException {
        return Files.readAllLines(Paths.get(hostsFile));
    }

    private static void writeHosts(List<String> lines) throws IOException {
        Files.write(Paths.get(hostsFile), lines);
    }

    private static void updateHosts(List<String> domains, String action) throws IOException {
        List<String> lines = readHosts();
        List<String> newLines = lines.stream()
            .filter(line -> {
                for (String d : domains) {
                    if (line.trim().startsWith(BLOCK_IP) && line.contains(d))
                        return false;
                }
                return true;
            })
            .collect(Collectors.toList());
        if (action.equals("add")) {
            for (String d : domains) newLines.add(BLOCK_IP + " " + d);
        }
        writeHosts(newLines);
    }

    private static void startBlocking(List<String> domains, int minutes) throws IOException {
        Config cfg = loadConfig();
        for (String d : domains) {
            if (!cfg.blocked.contains(d)) cfg.blocked.add(d);
        }
        if (minutes > 0) {
            cfg.timerEnd = Instant.now().plus(Duration.ofMinutes(minutes)).toString();
        }
        cfg.active = true;
        saveConfig(cfg);
        updateHosts(domains, "add");
        System.out.println(colorize("Blocked " + domains.size() + " domains.", GREEN));
        if (minutes > 0) {
            System.out.println(colorize("Blocking active for " + minutes + " min.", YELLOW));
            new Thread(() -> timerThread(minutes)).start();
        }
    }

    private static void timerThread(int minutes) {
        try {
            Thread.sleep(minutes * 60000L);
            System.out.println(colorize("Time expired. Unblocking...", YELLOW));
            disableBlocking();
        } catch (InterruptedException | IOException e) {
            System.err.println(colorize("Error: " + e.getMessage(), RED));
        }
    }

    private static void disableBlocking() throws IOException {
        Config cfg = loadConfig();
        if (cfg.blocked.isEmpty()) {
            System.out.println(colorize("No active blocking.", YELLOW));
            return;
        }
        updateHosts(cfg.blocked, "remove");
        cfg.blocked.clear();
        cfg.active = false;
        cfg.timerEnd = null;
        saveConfig(cfg);
        System.out.println(colorize("Blocking disabled.", GREEN));
    }

    private static void showStatus() throws IOException {
        Config cfg = loadConfig();
        if (cfg.active && cfg.timerEnd != null) {
            Instant end = Instant.parse(cfg.timerEnd);
            long remaining = Duration.between(Instant.now(), end).getSeconds();
            if (remaining > 0) {
                long mins = remaining / 60;
                long secs = remaining % 60;
                System.out.println(colorize("Blocking active. Remaining: " + mins + " min " + secs + " sec", BLUE));
            } else {
                System.out.println(colorize("Blocking active but timer expired.", YELLOW));
            }
        } else if (cfg.active) {
            System.out.println(colorize("Blocking active (no timer).", BLUE));
        } else {
            System.out.println(colorize("Blocking inactive.", YELLOW));
        }
        if (!cfg.blocked.isEmpty()) {
            System.out.println(colorize("Blocked domains (" + cfg.blocked.size() + "):", GREEN));
            for (String d : cfg.blocked) System.out.println("  - " + d);
        } else {
            System.out.println(colorize("No blocked domains.", YELLOW));
        }
    }

    public static void main(String[] args) throws IOException {
        if (args.length < 1) {
            System.out.println(colorize("Usage: blocker add|remove|list|enable|disable|status|pomodoro|clear [domains...] [options]", YELLOW));
            return;
        }
        String cmd = args[0];
        List<String> rest = new ArrayList<>();
        for (int i = 1; i < args.length; i++) rest.add(args[i]);

        ensureConfig();
        Config cfg = loadConfig();

        switch (cmd) {
            case "add": {
                if (rest.isEmpty()) {
                    System.out.println(colorize("Specify domains to block.", YELLOW));
                    return;
                }
                int idx = rest.indexOf("-t");
                int minutes = 0;
                if (idx != -1 && rest.size() > idx+1) {
                    minutes = Integer.parseInt(rest.get(idx+1));
                    rest.subList(idx, idx+2).clear();
                }
                startBlocking(rest, minutes);
                break;
            }
            case "remove": {
                if (rest.isEmpty()) {
                    System.out.println(colorize("Specify domains to remove.", YELLOW));
                    return;
                }
                for (String d : rest) cfg.blocked.remove(d);
                saveConfig(cfg);
                updateHosts(rest, "remove");
                System.out.println(colorize("Removed: " + String.join(", ", rest), GREEN));
                break;
            }
            case "list": {
                if (cfg.blocked.isEmpty())
                    System.out.println(colorize("No blocked domains.", YELLOW));
                else {
                    System.out.println(colorize("Blocked domains:", GREEN));
                    for (String d : cfg.blocked) System.out.println("  - " + d);
                }
                break;
            }
            case "enable": {
                if (cfg.blocked.isEmpty()) {
                    System.out.println(colorize("No domains in blocklist. Add them with 'add'.", YELLOW));
                    return;
                }
                int timeIdx = rest.indexOf("-t");
                int dur = 0;
                if (timeIdx != -1 && rest.size() > timeIdx+1)
                    dur = Integer.parseInt(rest.get(timeIdx+1));
                startBlocking(new ArrayList<>(cfg.blocked), dur);
                break;
            }
            case "disable": {
                disableBlocking();
                break;
            }
            case "status": {
                showStatus();
                break;
            }
            case "pomodoro": {
                if (cfg.blocked.isEmpty()) {
                    System.out.println(colorize("No domains in blocklist. Add them with 'add'.", YELLOW));
                    return;
                }
                System.out.println(colorize("Starting Pomodoro: 25 minutes work", BLUE));
                startBlocking(new ArrayList<>(cfg.blocked), 25);
                try { Thread.sleep(25 * 60000); } catch (InterruptedException e) {}
                System.out.println(colorize("Pomodoro finished! 5 minutes break.", GREEN));
                disableBlocking();
                try { Thread.sleep(5 * 60000); } catch (InterruptedException e) {}
                System.out.println(colorize("Break over. Start next Pomodoro.", BLUE));
                break;
            }
            case "clear": {
                boolean force = rest.contains("-f");
                if (!force) {
                    System.out.print(colorize("Clear all blocklist? [y/N] ", YELLOW));
                    Scanner sc = new Scanner(System.in);
                    String ans = sc.nextLine();
                    if (!ans.equalsIgnoreCase("y")) return;
                }
                if (cfg.blocked.isEmpty()) {
                    System.out.println(colorize("Blocklist already empty.", YELLOW));
                    return;
                }
                updateHosts(cfg.blocked, "remove");
                cfg.blocked.clear();
                cfg.active = false;
                cfg.timerEnd = null;
                saveConfig(cfg);
                System.out.println(colorize("Blocklist cleared.", GREEN));
                break;
            }
            default:
                System.out.println(colorize("Unknown command: " + cmd, RED));
        }
    }
}
