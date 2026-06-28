# blocker.py
#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import sys
import os
import json
import time
import argparse
import subprocess
import threading
from datetime import datetime, timedelta
from pathlib import Path

# ANSI colors
COLORS = {
    'green': '\033[92m',
    'red': '\033[91m',
    'yellow': '\033[93m',
    'blue': '\033[94m',
    'reset': '\033[0m'
}

def colorize(text, color):
    return f"{COLORS.get(color, '')}{text}{COLORS['reset']}"

# Конфигурация
CONFIG_DIR = Path.home() / '.blocker'
CONFIG_FILE = CONFIG_DIR / 'config.json'
HOSTS_FILE = '/etc/hosts' if os.name != 'nt' else r'C:\Windows\System32\drivers\etc\hosts'
BLOCK_IP = '127.0.0.1'

def ensure_config():
    CONFIG_DIR.mkdir(exist_ok=True)
    if not CONFIG_FILE.exists():
        default = {'blocked': [], 'whitelist': [], 'active': False, 'timer_end': None}
        with open(CONFIG_FILE, 'w') as f:
            json.dump(default, f)

def load_config():
    ensure_config()
    with open(CONFIG_FILE, 'r') as f:
        return json.load(f)

def save_config(config):
    with open(CONFIG_FILE, 'w') as f:
        json.dump(config, f, indent=2)

def read_hosts():
    try:
        with open(HOSTS_FILE, 'r') as f:
            return f.readlines()
    except PermissionError:
        raise PermissionError("Требуются права администратора для изменения hosts-файла")

def write_hosts(lines):
    try:
        with open(HOSTS_FILE, 'w') as f:
            f.writelines(lines)
    except PermissionError:
        raise PermissionError("Требуются права администратора для изменения hosts-файла")

def get_blocked_domains():
    # Извлекаем заблокированные домены из hosts-файла
    lines = read_hosts()
    blocked = []
    for line in lines:
        if line.startswith(BLOCK_IP) and not line.strip().startswith('#'):
            parts = line.split()
            if len(parts) >= 2:
                domain = parts[1]
                blocked.append(domain)
    return set(blocked)

def update_hosts(domains, action='add'):
    """Добавляет или удаляет домены из hosts-файла."""
    lines = read_hosts()
    # Фильтруем существующие записи для этих доменов (удаляем)
    new_lines = []
    for line in lines:
        if any(line.strip().startswith(BLOCK_IP) and domain in line for domain in domains):
            continue
        new_lines.append(line)
    if action == 'add':
        for domain in domains:
            new_lines.append(f"{BLOCK_IP} {domain}\n")
    write_hosts(new_lines)

def start_blocking(domains, duration_minutes=None):
    """Активирует блокировку для указанных доменов с таймером."""
    config = load_config()
    config['blocked'] = list(set(config['blocked'] + domains))
    if duration_minutes:
        config['timer_end'] = (datetime.now() + timedelta(minutes=duration_minutes)).isoformat()
    config['active'] = True
    save_config(config)
    # Обновляем hosts
    try:
        update_hosts(domains, 'add')
        print(colorize(f"Заблокировано {len(domains)} доменов.", 'green'))
        if duration_minutes:
            print(colorize(f"Блокировка активна {duration_minutes} мин.", 'yellow'))
            # Запускаем таймер в фоне
            threading.Thread(target=timer_thread, args=(duration_minutes,), daemon=True).start()
    except PermissionError as e:
        print(colorize(f"Ошибка: {e}", 'red'))
        sys.exit(1)

def timer_thread(minutes):
    """Фоновый таймер для автоматической разблокировки."""
    time.sleep(minutes * 60)
    print(colorize("Время блокировки истекло. Разблокировка...", 'yellow'))
    disable_blocking()

def disable_blocking():
    """Деактивирует блокировку (удаляет все записи из hosts)."""
    config = load_config()
    blocked = config.get('blocked', [])
    if blocked:
        try:
            update_hosts(blocked, 'remove')
            config['blocked'] = []
            config['active'] = False
            config['timer_end'] = None
            save_config(config)
            print(colorize("Блокировка отключена.", 'green'))
        except PermissionError as e:
            print(colorize(f"Ошибка: {e}", 'red'))
    else:
        print(colorize("Нет активной блокировки.", 'yellow'))

def show_status():
    config = load_config()
    blocked = config.get('blocked', [])
    active = config.get('active', False)
    timer_end = config.get('timer_end')
    if active and timer_end:
        end_time = datetime.fromisoformat(timer_end)
        remaining = (end_time - datetime.now()).seconds
        if remaining > 0:
            mins = remaining // 60
            secs = remaining % 60
            print(colorize(f"Блокировка активна. Осталось: {mins} мин {secs} сек", 'blue'))
        else:
            print(colorize("Блокировка активна, но таймер истёк.", 'yellow'))
    elif active:
        print(colorize("Блокировка активна (без таймера).", 'blue'))
    else:
        print(colorize("Блокировка неактивна.", 'yellow'))
    if blocked:
        print(colorize(f"Заблокировано {len(blocked)} доменов:", 'green'))
        for d in blocked:
            print(f"  - {d}")
    else:
        print(colorize("Нет заблокированных доменов.", 'yellow'))

def main():
    parser = argparse.ArgumentParser(description="Site Blocker – блокировка отвлекающих сайтов")
    parser.add_argument('cmd', choices=['add', 'remove', 'list', 'enable', 'disable', 'status', 'pomodoro', 'clear'],
                        help='Команда')
    parser.add_argument('domains', nargs='*', help='Домены для блокировки')
    parser.add_argument('-t', '--time', type=int, help='Временная блокировка (минуты)')
    parser.add_argument('-s', '--schedule', help='Расписание (HH:MM-HH:MM)')
    parser.add_argument('-f', '--force', action='store_true', help='Принудительно')
    args = parser.parse_args()

    ensure_config()
    config = load_config()

    if args.cmd == 'add':
        if not args.domains:
            print(colorize("Укажите домены для блокировки.", 'yellow'))
            return
        start_blocking(args.domains, args.time)

    elif args.cmd == 'remove':
        if not args.domains:
            print(colorize("Укажите домены для удаления.", 'yellow'))
            return
        blocked = config.get('blocked', [])
        for d in args.domains:
            if d in blocked:
                blocked.remove(d)
        config['blocked'] = blocked
        save_config(config)
        # Обновляем hosts
        try:
            update_hosts(args.domains, 'remove')
            print(colorize(f"Удалены: {', '.join(args.domains)}", 'green'))
        except PermissionError as e:
            print(colorize(f"Ошибка: {e}", 'red'))

    elif args.cmd == 'list':
        blocked = get_blocked_domains()
        if blocked:
            print(colorize("Заблокированные домены:", 'green'))
            for d in sorted(blocked):
                print(f"  - {d}")
        else:
            print(colorize("Нет заблокированных доменов.", 'yellow'))

    elif args.cmd == 'enable':
        blocked = config.get('blocked', [])
        if not blocked:
            print(colorize("Нет доменов в блок-листе. Добавьте их через 'add'.", 'yellow'))
            return
        start_blocking(blocked, args.time)

    elif args.cmd == 'disable':
        disable_blocking()

    elif args.cmd == 'status':
        show_status()

    elif args.cmd == 'pomodoro':
        print(colorize("Запуск Pomodoro: 25 минут работы", 'blue'))
        start_blocking(config.get('blocked', []), 25)
        # Через 25 минут уведомление
        time.sleep(25 * 60)
        print(colorize("Pomodoro завершён! Перерыв 5 минут.", 'green'))
        disable_blocking()
        time.sleep(5 * 60)
        print(colorize("Перерыв окончен. Можно начинать следующий Pomodoro.", 'blue'))

    elif args.cmd == 'clear':
        if not args.force:
            confirm = input(colorize("Очистить весь список блокировки? [y/N] ", 'yellow'))
            if confirm.lower() != 'y':
                return
        blocked = config.get('blocked', [])
        if blocked:
            try:
                update_hosts(blocked, 'remove')
                config['blocked'] = []
                config['active'] = False
                save_config(config)
                print(colorize("Блок-лист очищен.", 'green'))
            except PermissionError as e:
                print(colorize(f"Ошибка: {e}", 'red'))
        else:
            print(colorize("Блок-лист уже пуст.", 'yellow'))

if __name__ == '__main__':
    main()
