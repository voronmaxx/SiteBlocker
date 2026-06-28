// blocker.cpp
#include <iostream>
#include <fstream>
#include <string>
#include <vector>
#include <map>
#include <sstream>
#include <filesystem>
#include <thread>
#include <chrono>
#include <algorithm>
#include <json/json.h> // sudo apt-get install libjsoncpp-dev

using namespace std;
namespace fs = std::filesystem;

const string RESET = "\033[0m";
const string GREEN = "\033[92m";
const string RED = "\033[91m";
const string YELLOW = "\033[93m";
const string BLUE = "\033[94m";

string colorize(const string& text, const string& color) {
    return color + text + RESET;
}

string getConfigDir() {
    const char* home = getenv("HOME");
    if (!home) home = getenv("USERPROFILE");
    return string(home) + "/.blocker";
}

string getConfigFile() {
    return getConfigDir() + "/config.json";
}

string getHostsFile() {
#ifdef _WIN32
    return "C:\\Windows\\System32\\drivers\\etc\\hosts";
#else
    return "/etc/hosts";
#endif
}

const string BLOCK_IP = "127.0.0.1";

Json::Value loadConfig() {
    fs::create_directories(getConfigDir());
    ifstream f(getConfigFile());
    Json::Value root;
    if (!f) {
        root["blocked"] = Json::arrayValue;
        root["active"] = false;
        root["whitelist"] = Json::arrayValue;
        return root;
    }
    f >> root;
    return root;
}

void saveConfig(const Json::Value& root) {
    ofstream f(getConfigFile());
    f << root.toStyledString();
}

vector<string> readHosts() {
    ifstream f(getHostsFile());
    vector<string> lines;
    string line;
    while (getline(f, line)) lines.push_back(line);
    return lines;
}

void writeHosts(const vector<string>& lines) {
    ofstream f(getHostsFile());
    for (const auto& line : lines) f << line << endl;
}

void updateHosts(const vector<string>& domains, const string& action) {
    auto lines = readHosts();
    vector<string> newLines;
    for (const auto& line : lines) {
        bool skip = false;
        for (const auto& d : domains) {
            if (line.find(BLOCK_IP) == 0 && line.find(d) != string::npos) {
                skip = true;
                break;
            }
        }
        if (!skip) newLines.push_back(line);
    }
    if (action == "add") {
        for (const auto& d : domains) {
            newLines.push_back(BLOCK_IP + " " + d);
        }
    }
    writeHosts(newLines);
}

void timerThread(int minutes) {
    this_thread::sleep_for(chrono::minutes(minutes));
    cout << colorize("Time expired. Unblocking...", YELLOW) << endl;
    disableBlocking();
}

void startBlocking(const vector<string>& domains, int minutes) {
    auto root = loadConfig();
    for (const auto& d : domains) {
        root["blocked"].append(d);
    }
    if (minutes > 0) {
        auto now = chrono::system_clock::now();
        time_t tt = chrono::system_clock::to_time_t(now + chrono::minutes(minutes));
        char buf[64];
        strftime(buf, sizeof(buf), "%Y-%m-%dT%H:%M:%S", localtime(&tt));
        root["timer_end"] = string(buf);
    }
    root["active"] = true;
    saveConfig(root);
    try {
        updateHosts(domains, "add");
        cout << colorize("Blocked " + to_string(domains.size()) + " domains.", GREEN) << endl;
        if (minutes > 0) {
            cout << colorize("Blocking active for " + to_string(minutes) + " min.", YELLOW) << endl;
            thread(timerThread, minutes).detach();
        }
    } catch (const exception& e) {
        cerr << colorize("Error: " + string(e.what()), RED) << endl;
    }
}

void disableBlocking() {
    auto root = loadConfig();
    vector<string> blocked;
    for (const auto& d : root["blocked"]) blocked.push_back(d.asString());
    if (blocked.empty()) {
        cout << colorize("No active blocking.", YELLOW) << endl;
        return;
    }
    try {
        updateHosts(blocked, "remove");
        root["blocked"] = Json::arrayValue;
        root["active"] = false;
        root["timer_end"] = "";
        saveConfig(root);
        cout << colorize("Blocking disabled.", GREEN) << endl;
    } catch (const exception& e) {
        cerr << colorize("Error: " + string(e.what()), RED) << endl;
    }
}

void showStatus() {
    auto root = loadConfig();
    if (root["active"].asBool() && root.isMember("timer_end") && !root["timer_end"].asString().empty()) {
        string endStr = root["timer_end"].asString();
        tm tm = {};
        stringstream ss(endStr);
        ss >> get_time(&tm, "%Y-%m-%dT%H:%M:%S");
        time_t end = mktime(&tm);
        int remaining = (int)difftime(end, time(nullptr));
        if (remaining > 0) {
            int mins = remaining / 60;
            int secs = remaining % 60;
            cout << colorize("Blocking active. Remaining: " + to_string(mins) + " min " + to_string(secs) + " sec", BLUE) << endl;
        } else {
            cout << colorize("Blocking active but timer expired.", YELLOW) << endl;
        }
    } else if (root["active"].asBool()) {
        cout << colorize("Blocking active (no timer).", BLUE) << endl;
    } else {
        cout << colorize("Blocking inactive.", YELLOW) << endl;
    }
    if (root["blocked"].size() > 0) {
        cout << colorize("Blocked domains (" + to_string(root["blocked"].size()) + "):", GREEN) << endl;
        for (const auto& d : root["blocked"]) {
            cout << "  - " << d.asString() << endl;
        }
    } else {
        cout << colorize("No blocked domains.", YELLOW) << endl;
    }
}

int main(int argc, char* argv[]) {
    if (argc < 2) {
        cout << colorize("Usage: blocker add|remove|list|enable|disable|status|pomodoro|clear [domains...] [options]", YELLOW) << endl;
        return 1;
    }
    string cmd = argv[1];
    vector<string> args;
    for (int i = 2; i < argc; ++i) args.push_back(argv[i]);

    auto root = loadConfig();

    if (cmd == "add") {
        if (args.empty()) {
            cout << colorize("Specify domains to block.", YELLOW) << endl;
            return 1;
        }
        int minutes = 0;
        for (size_t i = 0; i < args.size(); ++i) {
            if (args[i] == "-t" && i+1 < args.size()) {
                minutes = stoi(args[i+1]);
                args.erase(args.begin()+i, args.begin()+i+2);
                break;
            }
        }
        startBlocking(args, minutes);
    }
    else if (cmd == "remove") {
        if (args.empty()) {
            cout << colorize("Specify domains to remove.", YELLOW) << endl;
            return 1;
        }
        vector<string> blocked;
        for (const auto& d : root["blocked"]) blocked.push_back(d.asString());
        for (const auto& d : args) {
            auto it = find(blocked.begin(), blocked.end(), d);
            if (it != blocked.end()) blocked.erase(it);
        }
        root["blocked"] = Json::arrayValue;
        for (const auto& d : blocked) root["blocked"].append(d);
        saveConfig(root);
        try {
            updateHosts(args, "remove");
            cout << colorize("Removed: ", GREEN);
            for (const auto& d : args) cout << d << " ";
            cout << endl;
        } catch (const exception& e) {
            cerr << colorize("Error: " + string(e.what()), RED) << endl;
        }
    }
    else if (cmd == "list") {
        if (root["blocked"].size() == 0) {
            cout << colorize("No blocked domains.", YELLOW) << endl;
        } else {
            cout << colorize("Blocked domains:", GREEN) << endl;
            for (const auto& d : root["blocked"]) {
                cout << "  - " << d.asString() << endl;
            }
        }
    }
    else if (cmd == "enable") {
        if (root["blocked"].size() == 0) {
            cout << colorize("No domains in blocklist. Add them with 'add'.", YELLOW) << endl;
            return 1;
        }
        int minutes = 0;
        for (size_t i = 0; i < args.size(); ++i) {
            if (args[i] == "-t" && i+1 < args.size()) {
                minutes = stoi(args[i+1]);
                break;
            }
        }
        vector<string> blocked;
        for (const auto& d : root["blocked"]) blocked.push_back(d.asString());
        startBlocking(blocked, minutes);
    }
    else if (cmd == "disable") {
        disableBlocking();
    }
    else if (cmd == "status") {
        showStatus();
    }
    else if (cmd == "pomodoro") {
        if (root["blocked"].size() == 0) {
            cout << colorize("No domains in blocklist. Add them with 'add'.", YELLOW) << endl;
            return 1;
        }
        cout << colorize("Starting Pomodoro: 25 minutes work", BLUE) << endl;
        vector<string> blocked;
        for (const auto& d : root["blocked"]) blocked.push_back(d.asString());
        startBlocking(blocked, 25);
        this_thread::sleep_for(chrono::minutes(25));
        cout << colorize("Pomodoro finished! 5 minutes break.", GREEN) << endl;
        disableBlocking();
        this_thread::sleep_for(chrono::minutes(5));
        cout << colorize("Break over. Start next Pomodoro.", BLUE) << endl;
    }
    else if (cmd == "clear") {
        bool force = false;
        for (const auto& arg : args) if (arg == "-f") force = true;
        if (!force) {
            cout << colorize("Clear all blocklist? [y/N] ", YELLOW);
            string ans;
            getline(cin, ans);
            if (ans != "y" && ans != "Y") return 0;
        }
        if (root["blocked"].size() == 0) {
            cout << colorize("Blocklist already empty.", YELLOW) << endl;
            return 0;
        }
        vector<string> blocked;
        for (const auto& d : root["blocked"]) blocked.push_back(d.asString());
        try {
            updateHosts(blocked, "remove");
            root["blocked"] = Json::arrayValue;
            root["active"] = false;
            root["timer_end"] = "";
            saveConfig(root);
            cout << colorize("Blocklist cleared.", GREEN) << endl;
        } catch (const exception& e) {
            cerr << colorize("Error: " + string(e.what()), RED) << endl;
        }
    }
    else {
        cout << colorize("Unknown command: " + cmd, RED) << endl;
    }
    return 0;
}
