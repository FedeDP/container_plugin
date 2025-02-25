#include <plugin.h>
#include <libworker.h>
#include <chrono>

//////////////////////////
// Async capability
//////////////////////////

static std::unique_ptr<falcosecurity::async_event_handler>
        s_async_handler[ASYNC_HANDLER_MAX];

std::vector<std::string> my_plugin::get_async_events()
{
    return ASYNC_EVENT_NAMES;
}

std::vector<std::string> my_plugin::get_async_event_sources()
{
    return ASYNC_EVENT_SOURCES;
}

static inline uint64_t get_current_time_ns(int sec_shift)
{
    std::chrono::nanoseconds ns =
            std::chrono::system_clock::now().time_since_epoch();
    return ns.count();
}

void generate_async_event(const char *json, bool added, int async_type)
{
    falcosecurity::events::asyncevent_e_encoder enc;
    enc.set_tid(1);
    std::string msg = json;
    if(added)
    {
        // leave ts=-1 (default value) to ensure that the event is grabbed asap
        enc.set_name(ASYNC_EVENT_NAME_ADDED);
    }
    else
    {
        // set ts = now + 1s to leave some space for enqueued syscalls to be
        // enriched
        enc.set_ts(get_current_time_ns(1));
        enc.set_name(ASYNC_EVENT_NAME_REMOVED);
    }
    enc.set_data((void *)msg.c_str(), msg.size() + 1);

    enc.encode(s_async_handler[async_type]->writer());
    s_async_handler[async_type]->push();
}

// We need this API to start the async thread when the
// `set_async_event_handler` plugin API will be called.
bool my_plugin::start_async_events(
        std::shared_ptr<falcosecurity::async_event_handler_factory> f)
{
    for(int i = 0; i < ASYNC_HANDLER_MAX; i++)
    {
        s_async_handler[i] = std::move(f->new_handler());
    }

    // Implemented by GO worker.go
    SPDLOG_DEBUG("starting async go-worker");
    nlohmann::json j(m_cfg);
    return StartWorker(generate_async_event, j.dump().c_str(),
                       ASYNC_HANDLER_GO_WORKER);
}

// We need this API to stop the async thread when the
// `set_async_event_handler` plugin API will be called.
bool my_plugin::stop_async_events() noexcept
{
    SPDLOG_DEBUG("stopping async go-worker");
    // Implemented by GO worker.go
    StopWorker();
    return true;
}

void my_plugin::dump(
        std::unique_ptr<falcosecurity::async_event_handler> async_handler)
{
    SPDLOG_DEBUG("dumping plugin internal state: {} containers",
                 m_containers.size());
    for(const auto &container : m_containers)
    {
        falcosecurity::events::asyncevent_e_encoder enc;
        enc.set_tid(1);
        nlohmann::json j(container.second);
        std::string msg = j.dump();
        enc.set_name(ASYNC_EVENT_NAME_ADDED);
        enc.set_data((void *)msg.c_str(), msg.size() + 1);

        enc.encode(async_handler->writer());
        async_handler->push();
    }
}

FALCOSECURITY_PLUGIN_ASYNC_EVENTS(my_plugin);
