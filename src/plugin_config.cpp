#include <filesystem>
#include "plugin_config.h"

void from_json(const nlohmann::json& j, StaticEngine& engine) {
    engine.enabled = j.value("enabled", false);
    engine.name = j.value("container_name", "");
    engine.id = j.value("container_id", "");
    engine.image = j.value("container_image", "");
}

void from_json(const nlohmann::json& j, SimpleEngine& engine) {
    engine.enabled = j.value("enabled", true);
}

void from_json(const nlohmann::json& j, SocketsEngine& engine) {
    engine.enabled = j.value("enabled", true);
    engine.sockets = j.value("sockets", std::vector<std::string>{});
}

void from_json(const nlohmann::json& j, PluginConfig& cfg) {
    cfg.verbosity = j.value("verbosity", "info");
    cfg.label_max_len = j.value("label_max_len", DEFAULT_LABEL_MAX_LEN);
    cfg.bpm = j.value("bpm", SimpleEngine{});
    cfg.lxc = j.value("lxc", SimpleEngine{});
    cfg.libvirt_lxc = j.value("libvirt_lxc", SimpleEngine{});
    cfg.static_ctr = j.value("static", StaticEngine{});

    cfg.docker = j.value("docker", SocketsEngine{});
    if (cfg.docker.sockets.empty()) {
        cfg.docker.sockets.emplace_back("/var/run/docker.sock");
    }

    cfg.podman = j.value("podman", SocketsEngine{});
    if (cfg.podman.sockets.empty()) {
        cfg.podman.sockets.emplace_back("/run/podman/podman.sock");
        for (const auto & entry : std::filesystem::directory_iterator("/run/user")) {
            if (entry.is_directory()) {
                if (std::filesystem::exists(entry.path().string() + "/podman/podman.sock")) {
                    cfg.podman.sockets.emplace_back(entry.path().string() + "/podman/podman.sock");
                }
            }
        }
    }

    cfg.cri = j.value("cri", SocketsEngine{});
    if (cfg.cri.sockets.empty()) {
        cfg.cri.sockets.emplace_back("/run/crio/crio.sock");
    }

    cfg.containerd = j.value("containerd", SocketsEngine{});
    if (cfg.containerd.sockets.empty()) {
        cfg.containerd.sockets.emplace_back("/run/containerd/containerd.sock");
        cfg.containerd.sockets.emplace_back("/run/k3s/containerd/containerd.sock");
    }
}

void to_json(nlohmann::json& j, const PluginConfig& cfg)
{
    j["label_max_len"] = cfg.label_max_len;
    j["engines"] = nlohmann::json{
            {
                    "docker",
                    {
                            {"enabled", cfg.docker.enabled },
                            {"sockets", cfg.docker.sockets }
                    }
            },
            {
                    "podman",
                    {
                            {"enabled", cfg.podman.enabled },
                            {"sockets", cfg.podman.sockets }
                    }
            },
            {
                    "cri",
                    {
                            {"enabled", cfg.cri.enabled },
                            {"sockets", cfg.cri.sockets }
                    }
            },
            {
                    "containerd",
                    {
                            {"enabled", cfg.containerd.enabled },
                            {"sockets", cfg.containerd.sockets }
                    }
            }
    };
}
