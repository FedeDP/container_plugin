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
#include "plugin.h"
#include "worker.h"

#include <re2/re2.h>

// This is the regex needed to extract the container_id from the cgroup
static re2::RE2 pattern(RGX_CONTAINER, re2::RE2::POSIX);

static std::unique_ptr<falcosecurity::async_event_handler> async_handler;

// TODO: rewrite for container_id
bool get_container_id_from_cgroup_string(const std::string& cgroup_first_line, std::string &container_id)
{
    if(re2::RE2::PartialMatch(cgroup_first_line, pattern, &container_id))
    {
        container_id.erase(0, 3);
        std::replace(container_id.begin(), container_id.end(), '_', '-');
        return true;
    }
    return false;
}

//////////////////////////
// General plugin API
//////////////////////////

falcosecurity::init_schema my_plugin::get_init_schema()
{
    falcosecurity::init_schema init_schema;
    init_schema.schema_type =
            falcosecurity::init_schema_type::SS_PLUGIN_SCHEMA_JSON;
    init_schema.schema = R"(
{
	"$schema": "http://json-schema.org/draft-04/schema#",
	"required": [],
	"properties": {
		"verbosity": {
			"enum": [
				"trace",
				"debug",
				"info",
				"warning",
				"error",
				"critical"
			],
			"title": "The plugin logging verbosity",
			"description": "The verbosity that the plugin will use when printing logs."
		}
	},
	"additionalProperties": false,
	"type": "object"
})";
    return init_schema;
}

void my_plugin::parse_init_config(nlohmann::json& config_json)
{
    // Verbosity, the default verbosity is already set in the 'init' method
    if(config_json.contains(nlohmann::json::json_pointer(VERBOSITY_PATH)))
    {
        // If the user specified a verbosity we override the actual one (`info`)
        std::string verbosity;
        config_json.at(nlohmann::json::json_pointer(VERBOSITY_PATH))
                .get_to(verbosity);
        spdlog::set_level(spdlog::level::from_str(verbosity));
    }
}

bool my_plugin::init(falcosecurity::init_input& in)
{
    using st = falcosecurity::state_value_type;
    auto& t = in.tables();

    // The default logger is already multithread.
    // The initial verbosity is `info`, after parsing the plugin config, this
    // value could change
    spdlog::set_level(spdlog::level::info);

    // Alternatives logs:
    // spdlog::set_pattern("%a %b %d %X %Y: [%l] [container] %v");
    //
    // We use local time like in Falco, not UTC
    spdlog::set_pattern("%c: [%l] [container] %v");

    // This should never happen, the config is validated by the framework
    if(in.get_config().empty())
    {
        m_lasterr = "cannot find the init config for the plugin";
        SPDLOG_CRITICAL(m_lasterr);
        return false;
    }

    auto cfg = nlohmann::json::parse(in.get_config());
    parse_init_config(cfg);

    SPDLOG_DEBUG("init the plugin");

    // Remove this log when we reach `1.0.0`
    SPDLOG_WARN("[EXPERIMENTAL] This plugin is in active development "
                "and may undergo changes in behavior without prioritizing "
                "backward compatibility.");

    try
    {
        m_threads_table = t.get_table(THREAD_TABLE_NAME, st::SS_PLUGIN_ST_INT64);

        // get the 'cgroups' field accessor from the thread table
        m_threads_field_cgroups = m_threads_table.get_field(
                t.fields(), CGROUPS_TABLE_NAME, st::SS_PLUGIN_ST_TABLE);

        // get the 'second' field accessor from the cgroups table
        m_cgroups_field_second = t.get_subtable_field(
                m_threads_table, m_threads_field_cgroups, "second",
                st::SS_PLUGIN_ST_STRING);

        // Add the container_id field into thread table
        m_container_id_field = m_threads_table.add_field(
                t.fields(), CONTAINER_ID_FIELD_NAME, st::SS_PLUGIN_ST_STRING);
    }
    catch(falcosecurity::plugin_exception e)
    {
        m_lasterr = "cannot add the '" + std::string(CONTAINER_ID_FIELD_NAME) +
                    "' field into the '" + std::string(THREAD_TABLE_NAME) +
                    "' table: " + e.what();
        SPDLOG_CRITICAL(m_lasterr);
        return false;
    }

    m_containers[HOST_CONTAINER_ID] = sinsp_container_info::host_container_info();
    return true;
}

//////////////////////////
// Async capability
//////////////////////////

static void generate_async_event(const char *json, bool added) {
    falcosecurity::events::asyncevent_e_encoder enc;
    // TODO: TID?
    enc.set_tid(1);
    std::string msg = json;
    if (added) {
        enc.set_name(ASYNC_EVENT_NAME_ADDED);
    } else {
        enc.set_name(ASYNC_EVENT_NAME_REMOVED);
    }
    enc.set_data((void*)msg.c_str(), msg.size() + 1);
    enc.encode(async_handler->writer());
    async_handler->push();
}

// We need this API to start the async thread when the
// `set_async_event_handler` plugin API will be called.
bool my_plugin::start_async_events(
        std::shared_ptr<falcosecurity::async_event_handler_factory> f) {
    async_handler = f->new_handler();
    // Implemented by GO worker.go
    // TODO: as soon as started, it should collect all pre-existing containers and
    // run callback on each of them, synchronously.
    StartWorker(generate_async_event);
    return true;
}

// We need this API to stop the async thread when the
// `set_async_event_handler` plugin API will be called.
bool my_plugin::stop_async_events() noexcept {
    // Implemented by GO worker.go
    StopWorker();
    async_handler.reset();
    return true;
}

//////////////////////////
// Extract capability
//////////////////////////

std::vector<falcosecurity::field_info> my_plugin::get_fields()
{
    using ft = falcosecurity::field_value_type;
    // Use an array to perform a static_assert one the size.
    const falcosecurity::field_info fields[] = {
            {ft::FTYPE_STRING, "container.id", "Container ID",
            "The truncated container ID (first 12 characters), e.g. 3ad7b26ded6d is extracted from "
            "the Linux cgroups by Falco within the kernel. Consequently, this field is reliably "
            "available and serves as the lookup key for Falco's synchronous or asynchronous requests "
            "against the container runtime socket to retrieve all other 'container.*' information. "
            "One important aspect to be aware of is that if the process occurs on the host, meaning "
            "not in the container PID namespace, this field is set to a string called 'host'. In "
            "Kubernetes, pod sandbox container processes can exist where `container.id` matches "
            "`k8s.pod.sandbox_id`, lacking other 'container.*' details.",
            {}, false, {}, true}, // use as suggested output format
            {ft::FTYPE_STRING, "container.full_id", "Container ID",
            "The full container ID, e.g. "
            "3ad7b26ded6d8e7b23da7d48fe889434573036c27ae5a74837233de441c3601e. In contrast to "
            "`container.id`, we enrich this field as part of the container engine enrichment. In "
            "instances of userspace container engine lookup delays, this field may not be available "
            "yet."},
            {ft::FTYPE_STRING, "container.name", "Container Name",
            "The container name. In instances of userspace container engine lookup delays, this field "
            "may not be available yet. One important aspect to be aware of is that if the process "
            "occurs on the host, meaning not in the container PID namespace, this field is set to a "
            "string called 'host'."
            , {}, false, {}, true}, // use as suggested output format
    };
    const int fields_size = sizeof(fields) / sizeof(fields[0]);
    // TODO: uncomment once all fields are exposed.
   // static_assert(fields_size == TYPE_CONTAINER_FIELD_MAX, "Wrong number of container fields.");
    return std::vector<falcosecurity::field_info>(fields, fields + fields_size);
}

bool my_plugin::extract(const falcosecurity::extract_fields_input& in)
{
    int64_t thread_id = in.get_event_reader().get_tid();
    if(thread_id <= 0)
    {
        SPDLOG_INFO("unknown thread id for event num '{}' with type '{}'",
                    in.get_event_reader().get_num(),
                    int32_t(in.get_event_reader().get_type()));
        return false;
    }

    std::string container_id = "";
    try
    {
        auto& tr = in.get_table_reader();
        // retrieve the thread entry associated with this thread id
        auto thread_entry = m_threads_table.get_entry(tr, thread_id);
        // retrieve container_id from the entry
        m_container_id_field.read_value(tr, thread_entry, container_id);
        if (container_id == "") {
            // This should only happen in case a clone/fork was lost
            // and our parse_new_process_event() callback was not called.
            // In this (rare) case, compute now the container_id given the thread cgroups
            container_id = compute_container_id_for_thread(thread_id, tr);
        }
    }
    catch(falcosecurity::plugin_exception e)
    {
        SPDLOG_ERROR("cannot extract the container_id for the thread id '{}': {}",
                     thread_id, e.what());
        return false;
    }

    // Try to find the entry associated with the pod_uid
    auto it = m_containers.find(container_id);
    if(it == m_containers.end())
    {
        SPDLOG_DEBUG("the plugin has no info for the container id '{}'", container_id);
        return false;
    }

    auto container_info = it->second;
    auto& req = in.get_extract_request();
    switch(req.get_field_id())
    {
    case TYPE_CONTAINER_ID:
        req.set_value(container_info.m_id, true);
        break;
    default:
        SPDLOG_ERROR(
                "unknown extraction request on field '{}' for container_id '{}'",
                req.get_field_id(), container_id);
        return false;
    }
    return true;
}

//////////////////////////
// Parse capability
//////////////////////////

// Obtain a param from a sinsp event
static inline sinsp_param get_syscall_evt_param(void* evt, uint32_t num_param)
{
    uint32_t dataoffset = 0;
    // pointer to the lengths array inside the event.
    auto len = (uint16_t*)((uint8_t*)evt +
                           sizeof(falcosecurity::_internal::ss_plugin_event));
    for(uint32_t j = 0; j < num_param; j++)
    {
        // sum lengths of the previous params.
        dataoffset += len[j];
    }
    return {.param_len = len[num_param],
            .param_pointer =
            ((uint8_t*)&len
            [((falcosecurity::_internal::ss_plugin_event*)evt)
                            ->nparams]) +
            dataoffset};
}

bool inline my_plugin::parse_async_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    falcosecurity::events::asyncevent_e_decoder ad(evt);
    bool added = std::strcmp(ad.get_name(), ASYNC_EVENT_NAME_ADDED) == 0;
    bool removed = std::strcmp(ad.get_name(), ASYNC_EVENT_NAME_REMOVED) == 0;
    if(!added && !removed)
    {
        // We are not interested in parsing async events that are not
        // generated by our plugin.
        // This is not an error, it could happen when we have more than one
        // async plugin loaded.
        SPDLOG_DEBUG("received an sync event with name {}", ad.get_name());
        return true;
    }

    uint32_t json_charbuf_len = 0;
    char* json_charbuf_pointer = (char*)ad.get_data(json_charbuf_len);
    if(json_charbuf_pointer == nullptr)
    {
        m_lasterr = "there is no payload in the async event";
        SPDLOG_ERROR(m_lasterr);
        return false;
    }
    auto json_event = nlohmann::json::parse(std::string(json_charbuf_pointer));

    auto container_info = sinsp_container_info::from_json(json_event);
    if (added) {
        m_containers[container_info.m_id] = container_info;
    } else {
        m_containers.erase(container_info.m_id);
    }
    return true;
}

bool inline my_plugin::parse_container_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    auto id_param = get_syscall_evt_param(evt.get_buf(), 0);
    auto type_param = get_syscall_evt_param(evt.get_buf(), 1);
    auto name_param = get_syscall_evt_param(evt.get_buf(), 2);
    auto image_param = get_syscall_evt_param(evt.get_buf(), 3);

    std::string id = (char*)id_param.param_pointer;
    sinsp_container_type tp = *((sinsp_container_type*)type_param.param_pointer);
    std::string name = (char*)name_param.param_pointer;
    std::string image = (char*)image_param.param_pointer;

    auto container_info = sinsp_container_info();
    container_info.m_id = id;
    container_info.m_type = tp;
    container_info.m_name = name;
    container_info.m_image = image;
    m_containers[id] = container_info;
    return true;
}

bool inline my_plugin::parse_container_json_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    auto json_param = get_syscall_evt_param(evt.get_buf(), 0);

    std::string json_str = (char *)json_param.param_pointer;
    auto json_event = nlohmann::json::parse(json_str);

    auto container_info = sinsp_container_info::from_json(json_event);
    m_containers[container_info.m_id] = container_info;
    return true;
}

std::string inline my_plugin::compute_container_id_for_thread(int64_t thread_id, const falcosecurity::table_reader& tr) {
    // retrieve tid cgroups, compute container_id and store it.
    std::string container_id;
    using st = falcosecurity::state_value_type;

    // retrieve the thread entry associated with this thread id
    auto thread_entry = m_threads_table.get_entry(tr, thread_id);

    // get the fd table of the thread
    auto cgroups_table = m_threads_table.get_subtable(
            tr, m_threads_field_cgroups, thread_entry,
            st::SS_PLUGIN_ST_UINT64);

    cgroups_table.iterate_entries(
            tr,
            [&](const falcosecurity::table_entry& e)
            {
                // read the "second" field (aka: the cgroup path)
                // from the current entry of the cgroups table
                std::string cgroup;
                m_cgroups_field_second.read_value(tr, e, cgroup);

                if(!cgroup.empty()) {
                    if (get_container_id_from_cgroup_string(cgroup, container_id)) {
                        return false; // stop iterating
                    }
                }
                return true;
            }
    );
    if (container_id == "") {
        // Could not find any matching container_id; HOST!
        container_id = HOST_CONTAINER_ID;
    }
    return container_id;
}

bool inline my_plugin::parse_new_process_event(
        const falcosecurity::parse_event_input& in) {
    // get tid
    int64_t thread_id = in.get_event_reader().get_tid();
    if(thread_id <= 0)
    {
        SPDLOG_INFO("unknown thread id for event num '{}' with type '{}'",
                    in.get_event_reader().get_num(),
                    int32_t(in.get_event_reader().get_type()));
        return false;
    }

    // compute container_id from tid->cgroups
    auto& tr = in.get_table_reader();
    auto container_id = compute_container_id_for_thread(thread_id, tr);

    // store container_id
    auto& tw = in.get_table_writer();
    // retrieve the thread entry associated with this thread id
    auto thread_entry = m_threads_table.get_entry(tr, thread_id);
    m_container_id_field.write_value(tw, thread_entry,
                                     (const char*)container_id.c_str());
    return true;
}

bool my_plugin::parse_event(const falcosecurity::parse_event_input& in)
{
    // NOTE: today in the libs framework, parsing errors are not logged
    auto& evt = in.get_event_reader();

    switch(evt.get_type())
    {
    case PPME_ASYNCEVENT_E:
        return parse_async_event(in);
    case PPME_CONTAINER_E:
        return parse_container_event(in);
    case PPME_CONTAINER_JSON_E:
    case PPME_CONTAINER_JSON_2_E:
        return parse_container_json_event(in);
    case PPME_SYSCALL_CLONE_20_X:
    case PPME_SYSCALL_FORK_20_X:
    case PPME_SYSCALL_VFORK_20_X:
    case PPME_SYSCALL_CLONE3_X:
        return parse_new_process_event(in);
    default:
        SPDLOG_ERROR("received an unknown event type {}",
                     int32_t(evt.get_type()));
        return false;
    }
}
