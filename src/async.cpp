#include "plugin.h"
#include <libworker.h>

//////////////////////////
// Async capability
//////////////////////////

using nlohmann::json;

static std::unique_ptr<falcosecurity::async_event_handler> s_async_handler;

std::vector<std::string> my_plugin::get_async_events() {
    return ASYNC_EVENT_NAMES;
}

std::vector<std::string> my_plugin::get_async_event_sources() {
    return ASYNC_EVENT_SOURCES;
}

static void generate_async_event(const char *json, bool added) {
    falcosecurity::events::asyncevent_e_encoder enc;
    enc.set_tid(1);
    std::string msg = json;
    if (added) {
        enc.set_name(ASYNC_EVENT_NAME_ADDED);
    } else {
        enc.set_name(ASYNC_EVENT_NAME_REMOVED);
    }
    enc.set_data((void*)msg.c_str(), msg.size() + 1);
    enc.encode(s_async_handler->writer());
    s_async_handler->push();
}

void to_json(json& j, const PluginConfig& cfg)
{
    j = json{
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


// We need this API to start the async thread when the
// `set_async_event_handler` plugin API will be called.
bool my_plugin::start_async_events(
        std::shared_ptr<falcosecurity::async_event_handler_factory> f) {
    s_async_handler = f->new_handler();
    // Implemented by GO worker.go

    json j(m_cfg);
    return StartWorker(generate_async_event, j.dump().c_str());
}

// We need this API to stop the async thread when the
// `set_async_event_handler` plugin API will be called.
bool my_plugin::stop_async_events() noexcept {
    // Implemented by GO worker.go
    StopWorker();
    s_async_handler.reset();
    return true;
}

FALCOSECURITY_PLUGIN_ASYNC_EVENTS(my_plugin);
