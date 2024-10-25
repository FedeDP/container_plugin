/*
Copyright (C) 2023 The Falco Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

#include "plugin_only_consts.h"
#include "shared_with_tests_consts.h"
#include "container_info.h"
#include <thread>
#include <atomic>
#include <chrono>
#include <unordered_map>
#include <sstream>

struct sinsp_param
{
    uint16_t param_len;
    uint8_t* param_pointer;
};

class my_plugin
{
public:
    // Keep this aligned with `get_fields`
    enum ContainerFields
    {
        TYPE_CONTAINER_ID,
        TYPE_CONTAINER_FULL_CONTAINER_ID,
        TYPE_CONTAINER_NAME,
        TYPE_CONTAINER_IMAGE,
        TYPE_CONTAINER_IMAGE_ID,
        TYPE_CONTAINER_TYPE,
        TYPE_CONTAINER_PRIVILEGED,
        TYPE_CONTAINER_MOUNTS,
        TYPE_CONTAINER_MOUNT,
        TYPE_CONTAINER_MOUNT_SOURCE,
        TYPE_CONTAINER_MOUNT_DEST,
        TYPE_CONTAINER_MOUNT_MODE,
        TYPE_CONTAINER_MOUNT_RDWR,
        TYPE_CONTAINER_MOUNT_PROPAGATION,
        TYPE_CONTAINER_IMAGE_REPOSITORY,
        TYPE_CONTAINER_IMAGE_TAG,
        TYPE_CONTAINER_IMAGE_DIGEST,
        TYPE_CONTAINER_HEALTHCHECK,
        TYPE_CONTAINER_LIVENESS_PROBE,
        TYPE_CONTAINER_READINESS_PROBE,
        TYPE_CONTAINER_START_TS,
        TYPE_CONTAINER_DURATION,
        TYPE_CONTAINER_IP_ADDR,
        TYPE_CONTAINER_CNIRESULT,
        TYPE_CONTAINER_HOST_PID,
        TYPE_CONTAINER_HOST_NETWORK,
        TYPE_CONTAINER_HOST_IPC,
        TYPE_CONTAINER_FIELD_MAX
    };

    //////////////////////////
    // General plugin API
    //////////////////////////

    virtual ~my_plugin() = default;

    std::string get_name() { return PLUGIN_NAME; }

    std::string get_version() { return PLUGIN_VERSION; }

    std::string get_description() { return PLUGIN_DESCRIPTION; }

    std::string get_contact() { return PLUGIN_CONTACT; }

    std::string get_required_api_version()
    {
        return PLUGIN_REQUIRED_API_VERSION;
    }

    std::string get_last_error() { return m_lasterr; }

    void destroy() { SPDLOG_DEBUG("detach the plugin"); }

    falcosecurity::init_schema get_init_schema();

    void parse_init_config(nlohmann::json& config_json);

    bool init(falcosecurity::init_input& in);

    std::string inline compute_container_id_for_thread(int64_t thread_id, const falcosecurity::table_reader& tr);

    //////////////////////////
    // Async capability
    //////////////////////////

    std::vector<std::string> get_async_events() { return ASYNC_EVENT_NAMES; }

    std::vector<std::string> get_async_event_sources()
    {
        return ASYNC_EVENT_SOURCES;
    }

    bool start_async_events(
            std::shared_ptr<falcosecurity::async_event_handler_factory> f);

    bool stop_async_events() noexcept;

    void async_thread_loop(
            std::unique_ptr<falcosecurity::async_event_handler> h) noexcept;

    //////////////////////////
    // Extract capability
    //////////////////////////

    std::vector<std::string> get_extract_event_sources()
    {
        return EXTRACT_EVENT_SOURCES;
    }

    std::vector<falcosecurity::field_info> get_fields();

    bool extract(const falcosecurity::extract_fields_input& in);

    //////////////////////////
    // Parse capability
    //////////////////////////

    // We need to parse only the async events produced by this plugin. The async
    // events produced by this plugin are injected in the syscall event source,
    // so here we need to parse events coming from the "syscall" source.
    // We will select specific events to parse through the
    // `get_parse_event_types` API.
    std::vector<std::string> get_parse_event_sources()
    {
        return PARSE_EVENT_SOURCES;
    }

    std::vector<falcosecurity::event_type> get_parse_event_types()
    {
        return PARSE_EVENT_CODES;
    }

    bool inline parse_async_event(const falcosecurity::parse_event_input& in);
    bool inline parse_container_event(const falcosecurity::parse_event_input& in);
    bool inline parse_container_json_event(const falcosecurity::parse_event_input& in);
    bool inline parse_new_process_event(const falcosecurity::parse_event_input& in);
    bool parse_event(const falcosecurity::parse_event_input& in);

private:
    // Async thread
    std::thread m_async_thread;
    std::atomic<bool> m_async_thread_quit;
    std::condition_variable m_cv;
    std::mutex m_mu;

    // State tables
    std::unordered_map<std::string, sinsp_container_info> m_containers;

    // Last error of the plugin
    std::string m_lasterr;
    // Accessor to the thread table
    falcosecurity::table m_threads_table;
    // Accessors to the thread table "cgroups" table
    falcosecurity::table_field m_threads_field_cgroups;
    // Accessors to the thread table "cgroups" "second" field, ie: the cgroups path
    falcosecurity::table_field m_cgroups_field_second;
    // Accessors to the thread table "container_id" foreign key field
    falcosecurity::table_field m_container_id_field;
};

FALCOSECURITY_PLUGIN(my_plugin);
FALCOSECURITY_PLUGIN_FIELD_EXTRACTION(my_plugin);
FALCOSECURITY_PLUGIN_ASYNC_EVENTS(my_plugin);
FALCOSECURITY_PLUGIN_EVENT_PARSING(my_plugin);
