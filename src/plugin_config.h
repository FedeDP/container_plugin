#pragma once

#include <nlohmann/json.hpp>
#include <spdlog/spdlog.h>

#define DEFAULT_LABEL_MAX_LEN 100

struct SimpleEngine
{
    bool enabled;

    SimpleEngine() { enabled = true; }
};

struct SocketsEngine
{
    bool enabled;
    std::vector<std::string> sockets;

    SocketsEngine() { enabled = true; }

    void log_sockets(const std::string& host_root) const
    {
        for(const auto& socket : sockets)
        {
            SPDLOG_DEBUG("* enabled container runtime socket at '{}'",
                         host_root + socket);
        }
    }
};

struct StaticEngine
{
    bool enabled;
    std::string id;
    std::string name;
    std::string image;

    StaticEngine() { enabled = false; }
};

struct Engines
{
    SimpleEngine bpm;
    SimpleEngine lxc;
    SimpleEngine libvirt_lxc;
    SocketsEngine docker;
    SocketsEngine podman;
    SocketsEngine cri;
    SocketsEngine containerd;
    StaticEngine static_ctr;
};

struct PluginConfig
{
    std::string verbosity;
    int label_max_len;
    bool with_size;
    std::string host_root;
    Engines engines;

    PluginConfig()
    {
        label_max_len = DEFAULT_LABEL_MAX_LEN;
        with_size = false;
        if(const char* hroot = std::getenv("HOST_ROOT"))
        {
            host_root = hroot;
        }
    }
};

/* Nlhomann adapters (implemented by plugin_config.cpp) */

// from_json is used by parse_init_config() during plugin::init and just parses
// plugin config json string to a structure.
void from_json(const nlohmann::json& j, StaticEngine& engine);
void from_json(const nlohmann::json& j, SimpleEngine& engine);
void from_json(const nlohmann::json& j, SocketsEngine& engine);
void from_json(const nlohmann::json& j, Engines& engines);
void from_json(const nlohmann::json& j, PluginConfig& cfg);

// Build the json object to be passed to the go-worker as init config.
// See go-worker/engine.go::cfg struct for the format
void to_json(nlohmann::json& j, const Engines& engines);
void to_json(nlohmann::json& j, const PluginConfig& cfg);