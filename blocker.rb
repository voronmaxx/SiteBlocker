#!/usr/bin/env ruby
# blocker.rb
# encoding: UTF-8

require 'json'
require 'fileutils'
require 'time'

COLORS = {
  green: "\e[92m",
  red: "\e[91m",
  yellow: "\e[93m",
  blue: "\e[94m",
  reset: "\e[0m"
}

def colorize(text, color)
  "#{COLORS[color]}#{text}#{COLORS[:reset]}"
end

CONFIG_DIR = File.join(Dir.home, '.blocker')
CONFIG_FILE = File.join(CONFIG_DIR, 'config.json')
HOSTS_FILE = RUBY_PLATFORM =~ /mswin|mingw|cygwin/ ?
  'C:\Windows\System32\drivers\etc\hosts' : '/etc/hosts'
BLOCK_IP = '127.0.0.1'

def ensure_config
  FileUtils.mkdir_p(CONFIG_DIR)
  unless File.exist?(CONFIG_FILE)
    default = { 'blocked' => [], 'active' => false, 'whitelist' => [] }
    File.write(CONFIG_FILE, JSON.pretty_generate(default))
  end
end

def load_config
  ensure_config
  JSON.parse(File.read(CONFIG_FILE))
end

def save_config(cfg)
  File.write(CONFIG_FILE, JSON.pretty_generate(cfg))
end

def read_hosts
  File.readlines(HOSTS_FILE).map(&:chomp)
rescue Errno::EACCES
  raise "Требуются права администратора для изменения hosts-файла"
end

def write_hosts(lines)
  File.write(HOSTS_FILE, lines.join("\n") + "\n")
rescue Errno::EACCES
  raise "Требуются права администратора для изменения hosts-файла"
end

def update_hosts(domains, action)
  lines = read_hosts
  new_lines = lines.reject do |line|
    domains.any? { |d| line.strip.start_with?(BLOCK_IP) && line.include?(d) }
  end
  if action == 'add'
    domains.each { |d| new_lines << "#{BLOCK_IP} #{d}" }
  end
  write_hosts(new_lines)
end

def start_blocking(domains, minutes)
  cfg = load_config
  cfg['blocked'] = (cfg['blocked'] + domains).uniq
  if minutes && minutes > 0
    cfg['timer_end'] = (Time.now + minutes * 60).iso8601
  end
  cfg['active'] = true
  save_config(cfg)
  update_hosts(domains, 'add')
  puts colorize("Blocked #{domains.size} domains.", :green)
  if minutes && minutes > 0
    puts colorize("Blocking active for #{minutes} min.", :yellow)
    Thread.new { timer_thread(minutes) }
  end
end

def timer_thread(minutes)
  sleep(minutes * 60)
  puts colorize("Time expired. Unblocking...", :yellow)
  disable_blocking
end

def disable_blocking
  cfg = load_config
  if cfg['blocked'].empty?
    puts colorize("No active blocking.", :yellow)
    return
  end
  update_hosts(cfg['blocked'], 'remove')
  cfg['blocked'] = []
  cfg['active'] = false
  cfg['timer_end'] = nil
  save_config(cfg)
  puts colorize("Blocking disabled.", :green)
end

def show_status
  cfg = load_config
  if cfg['active'] && cfg['timer_end']
    remaining = (Time.iso8601(cfg['timer_end']) - Time.now).to_i
    if remaining > 0
      mins = remaining / 60
      secs = remaining % 60
      puts colorize("Blocking active. Remaining: #{mins} min #{secs} sec", :blue)
    else
      puts colorize("Blocking active but timer expired.", :yellow)
    end
  elsif cfg['active']
    puts colorize("Blocking active (no timer).", :blue)
  else
    puts colorize("Blocking inactive.", :yellow)
  end
  if cfg['blocked'].any?
    puts colorize("Blocked domains (#{cfg['blocked'].size}):", :green)
    cfg['blocked'].each { |d| puts "  - #{d}" }
  else
    puts colorize("No blocked domains.", :yellow)
  end
end

def main
  if ARGV.empty?
    puts colorize("Usage: blocker add|remove|list|enable|disable|status|pomodoro|clear [domains...] [options]", :yellow)
    exit 1
  end

  cmd = ARGV[0]
  rest = ARGV[1..-1] || []
  cfg = load_config

  case cmd
  when 'add'
    if rest.empty?
      puts colorize("Specify domains to block.", :yellow)
      return
    end
    minutes = 0
    idx = rest.index('-t')
    if idx && rest[idx+1]
      minutes = rest[idx+1].to_i
      rest.delete_at(idx)
      rest.delete_at(idx)
    end
    start_blocking(rest, minutes)

  when 'remove'
    if rest.empty?
      puts colorize("Specify domains to remove.", :yellow)
      return
    end
    cfg['blocked'] -= rest
    save_config(cfg)
    update_hosts(rest, 'remove')
    puts colorize("Removed: #{rest.join(', ')}", :green)

  when 'list'
    if cfg['blocked'].empty?
      puts colorize("No blocked domains.", :yellow)
    else
      puts colorize("Blocked domains:", :green)
      cfg['blocked'].each { |d| puts "  - #{d}" }
    end

  when 'enable'
    if cfg['blocked'].empty?
      puts colorize("No domains in blocklist. Add them with 'add'.", :yellow)
      return
    end
    minutes = 0
    idx = rest.index('-t')
    if idx && rest[idx+1]
      minutes = rest[idx+1].to_i
    end
    start_blocking(cfg['blocked'].dup, minutes)

  when 'disable'
    disable_blocking

  when 'status'
    show_status

  when 'pomodoro'
    if cfg['blocked'].empty?
      puts colorize("No domains in blocklist. Add them with 'add'.", :yellow)
      return
    end
    puts colorize("Starting Pomodoro: 25 minutes work", :blue)
    start_blocking(cfg['blocked'].dup, 25)
    sleep(25 * 60)
    puts colorize("Pomodoro finished! 5 minutes break.", :green)
    disable_blocking
    sleep(5 * 60)
    puts colorize("Break over. Start next Pomodoro.", :blue)

  when 'clear'
    force = rest.include?('-f')
    unless force
      print colorize("Clear all blocklist? [y/N] ", :yellow)
      ans = $stdin.gets.chomp
      return unless ans.downcase == 'y'
    end
    if cfg['blocked'].empty?
      puts colorize("Blocklist already empty.", :yellow)
      return
    end
    update_hosts(cfg['blocked'], 'remove')
    cfg['blocked'] = []
    cfg['active'] = false
    cfg['timer_end'] = nil
    save_config(cfg)
    puts colorize("Blocklist cleared.", :green)

  else
    puts colorize("Unknown command: #{cmd}", :red)
  end
end

main if __FILE__ == $0
