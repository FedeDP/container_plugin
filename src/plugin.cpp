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

#include "plugin.h"
#include <re2/re2.h>

//////////////////////////
// General plugin API
//////////////////////////

// This is the regex needed to extract the container_id from the cgroup
static re2::RE2 pattern(RGX_CONTAINER, re2::RE2::POSIX);

std::string my_plugin::get_name() {
    return PLUGIN_NAME;
}

std::string my_plugin::get_version() {
    return PLUGIN_VERSION;
}

std::string my_plugin::get_description() {
    return PLUGIN_DESCRIPTION;
}

std::string my_plugin::get_contact() {
    return PLUGIN_CONTACT;
}

std::string my_plugin::get_required_api_version() {
    return PLUGIN_REQUIRED_API_VERSION;
}

std::string my_plugin::get_last_error() {
    return m_lasterr;
}

void my_plugin::destroy() {
    SPDLOG_DEBUG("detach the plugin");
}

falcosecurity::init_schema my_plugin::get_init_schema() {
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

void my_plugin::parse_init_config(nlohmann::json& config_json) {
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

bool my_plugin::init(falcosecurity::init_input& in) {
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
    if(in.get_config().empty()) {
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

    try {
        m_threads_table = t.get_table(THREAD_TABLE_NAME, st::SS_PLUGIN_ST_INT64);

        m_threads_field_pidns_init_start_ts = m_threads_table.get_field(
                t.fields(), PIDNS_INIT_START_TS_FIELD_NAME, st::SS_PLUGIN_ST_UINT64);

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
    } catch(falcosecurity::plugin_exception e) {
        m_lasterr = "cannot add the '" + std::string(CONTAINER_ID_FIELD_NAME) +
                    "' field into the '" + std::string(THREAD_TABLE_NAME) +
                    "' table: " + e.what();
        SPDLOG_CRITICAL(m_lasterr);
        return false;
    }

    m_containers[HOST_CONTAINER_ID] = container_info::host_container_info();
    return true;
}

// TODO: rewrite for container_id
static bool inline get_container_id_from_cgroup_string(const std::string& cgroup_first_line, std::string &container_id) {
    if(re2::RE2::PartialMatch(cgroup_first_line, pattern, &container_id))
    {
        container_id.erase(0, 3);
        std::replace(container_id.begin(), container_id.end(), '_', '-');
        return true;
    }
    return false;
}

std::string my_plugin::compute_container_id_for_thread(int64_t thread_id, const falcosecurity::table_reader& tr) {
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

FALCOSECURITY_PLUGIN(my_plugin);