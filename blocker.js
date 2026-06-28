// blocker.js
#!/usr/bin/env node
'use strict';

const fs = require('fs');
const path = require('path');
const os = require('os');
const { execSync } = require('child_process');

const COLORS = {
    green: '\x1b[92m',
    red: '\x1b[91m',
    yellow: '\x1b[93m',
    blue: '\x1b[94m',
    reset: '\x1b[0m'
};

function colorize(text, color) {
    return COLORS[color] + text + COLORS.reset;
}

const CONFIG_DIR = path.join(os.homedir(), '.blocker');
const CONFIG_FILE = path.join(CONFIG_DIR, 'config.json');
const HOSTS_FILE = os.platform() === 'win32'
    ? 'C:\\Windows\\System32\\drivers\\etc\\hosts'
    : '/etc/hosts';
const BLOCK_IP = '127.0.0.1';

function ensureConfig() {
    if (!fs.existsSync(CONFIG_DIR)) fs.mkdirSync(CONFIG_DIR, { recursive: true });
    if (!fs.existsSync(CONFIG_FILE)) {
        const defaultConfig = { blocked: [], active: false, whitelist: [] };
        fs.writeFileSync(CONFIG_FILE, JSON.stringify(defaultConfig, null, 2));
    }
}

function loadConfig() {
    ensureConfig();
    return JSON.parse(fs.readFileSync(CONFIG_FILE, 'utf8'));
}

function saveConfig(cfg) {
    fs.writeFileSync(CONFIG_FILE, JSON.stringify(cfg, null, 2));
}

function readHosts() {
    return fs.readFileSync(HOSTS_FILE, 'utf8').split('\n');
}

function writeHosts(lines) {
    fs.writeFileSync(HOSTS_FILE, lines.join('\n'));
}

function updateHosts(domains, action) {
    let lines = readHosts();
    const filtered = lines.filter(line => {
        for (const d of domains) {
            if (line.trim().startsWith(BLOCK_IP) && line.includes(d)) return false;
        }
        return true;
    });
    if (action === 'add') {
        for (const d of domains) {
            filtered.push(`${BLOCK_IP} ${d}`);
        }
    }
    writeHosts(filtered);
}

function startBlocking(domains, durationMinutes) {
    const cfg = loadConfig();
    cfg.blocked = [...new Set([...cfg.blocked, ...domains])];
    if (durationMinutes) {
        cfg.timerEnd = new Date(Date.now() + durationMinutes * 60000).toISOString();
    }
    cfg.active = true;
    saveConfig(cfg);
    try {
        updateHosts(domains, 'add');
        console.log(colorize(`Blocked ${domains.length} domains.`, 'green'));
        if (durationMinutes) {
            console.log(colorize(`Blocking active for ${durationMinutes} min.`, 'yellow'));
            setTimeout(() => {
                console.log(colorize('Time expired. Unblocking...', 'yellow'));
                disableBlocking();
            }, durationMinutes * 60000);
        }
    } catch (err) {
        console.log(colorize(`Error: ${err.message}`, 'red'));
        process.exit(1);
    }
}

function disableBlocking() {
    const cfg = loadConfig();
    const blocked = cfg.blocked;
    if (blocked.length === 0) {
        console.log(colorize('No active blocking.', 'yellow'));
        return;
    }
    try {
        updateHosts(blocked, 'remove');
        cfg.blocked = [];
        cfg.active = false;
        cfg.timerEnd = null;
        saveConfig(cfg);
        console.log(colorize('Blocking disabled.', 'green'));
    } catch (err) {
        console.log(colorize(`Error: ${err.message}`, 'red'));
    }
}

function showStatus() {
    const cfg = loadConfig();
    if (cfg.active && cfg.timerEnd) {
        const remaining = Math.floor((new Date(cfg.timerEnd) - Date.now()) / 1000);
        if (remaining > 0) {
            const mins = Math.floor(remaining / 60);
            const secs = remaining % 60;
            console.log(colorize(`Blocking active. Remaining: ${mins} min ${secs} sec`, 'blue'));
        } else {
            console.log(colorize('Blocking active but timer expired.', 'yellow'));
        }
    } else if (cfg.active) {
        console.log(colorize('Blocking active (no timer).', 'blue'));
    } else {
        console.log(colorize('Blocking inactive.', 'yellow'));
    }
    if (cfg.blocked.length > 0) {
        console.log(colorize(`Blocked domains (${cfg.blocked.length}):`, 'green'));
        cfg.blocked.forEach(d => console.log(`  - ${d}`));
    } else {
        console.log(colorize('No blocked domains.', 'yellow'));
    }
}

function main() {
    const args = process.argv.slice(2);
    if (args.length < 1) {
        console.log(colorize('Usage: blocker add|remove|list|enable|disable|status|pomodoro|clear [domains...] [options]', 'yellow'));
        process.exit(1);
    }
    const cmd = args[0];
    const rest = args.slice(1);

    ensureConfig();
    const cfg = loadConfig();

    switch (cmd) {
        case 'add': {
            if (rest.length === 0) {
                console.log(colorize('Specify domains to block.', 'yellow'));
                return;
            }
            const timeIdx = rest.indexOf('-t');
            let duration = 0;
            if (timeIdx !== -1 && rest.length > timeIdx+1) {
                duration = parseInt(rest[timeIdx+1]);
                rest.splice(timeIdx, 2);
            }
            startBlocking(rest, duration);
            break;
        }
        case 'remove': {
            if (rest.length === 0) {
                console.log(colorize('Specify domains to remove.', 'yellow'));
                return;
            }
            const blocked = cfg.blocked;
            for (const d of rest) {
                const idx = blocked.indexOf(d);
                if (idx !== -1) blocked.splice(idx, 1);
            }
            saveConfig(cfg);
            try {
                updateHosts(rest, 'remove');
                console.log(colorize(`Removed: ${rest.join(', ')}`, 'green'));
            } catch (err) {
                console.log(colorize(`Error: ${err.message}`, 'red'));
            }
            break;
        }
        case 'list': {
            const blocked = cfg.blocked;
            if (blocked.length === 0) {
                console.log(colorize('No blocked domains.', 'yellow'));
            } else {
                console.log(colorize('Blocked domains:', 'green'));
                blocked.forEach(d => console.log(`  - ${d}`));
            }
            break;
        }
        case 'enable': {
            if (cfg.blocked.length === 0) {
                console.log(colorize('No domains in blocklist. Add them with "add".', 'yellow'));
                return;
            }
            const timeIdx = rest.indexOf('-t');
            let duration = 0;
            if (timeIdx !== -1 && rest.length > timeIdx+1) {
                duration = parseInt(rest[timeIdx+1]);
            }
            startBlocking(cfg.blocked, duration);
            break;
        }
        case 'disable': {
            disableBlocking();
            break;
        }
        case 'status': {
            showStatus();
            break;
        }
        case 'pomodoro': {
            if (cfg.blocked.length === 0) {
                console.log(colorize('No domains in blocklist. Add them with "add".', 'yellow'));
                return;
            }
            console.log(colorize('Starting Pomodoro: 25 minutes work', 'blue'));
            startBlocking(cfg.blocked, 25);
            setTimeout(() => {
                console.log(colorize('Pomodoro finished! 5 minutes break.', 'green'));
                disableBlocking();
                setTimeout(() => {
                    console.log(colorize('Break over. Start next Pomodoro.', 'blue'));
                }, 5 * 60000);
            }, 25 * 60000);
            break;
        }
        case 'clear': {
            const force = rest.includes('-f');
            if (!force) {
                const readline = require('readline').createInterface({
                    input: process.stdin,
                    output: process.stdout
                });
                readline.question(colorize('Clear all blocklist? [y/N] ', 'yellow'), ans => {
                    readline.close();
                    if (ans.toLowerCase() === 'y') doClear();
                });
            } else {
                doClear();
            }
            break;
        }
        default:
            console.log(colorize(`Unknown command: ${cmd}`, 'red'));
    }
}

function doClear() {
    const cfg = loadConfig();
    if (cfg.blocked.length === 0) {
        console.log(colorize('Blocklist already empty.', 'yellow'));
        return;
    }
    try {
        updateHosts(cfg.blocked, 'remove');
        cfg.blocked = [];
        cfg.active = false;
        cfg.timerEnd = null;
        saveConfig(cfg);
        console.log(colorize('Blocklist cleared.', 'green'));
    } catch (err) {
        console.log(colorize(`Error: ${err.message}`, 'red'));
    }
}

main();
