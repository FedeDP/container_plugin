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
#include "plugin_config_schema.h"

//////////////////////////
// General plugin API
//////////////////////////

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
    init_schema.schema = plugin_schema_string;
    return init_schema;
}

void my_plugin::parse_init_config(nlohmann::json& config_json) {
    m_cfg = config_json.get<PluginConfig>();
    // Verbosity, the default verbosity is already set in the 'init' method
    if (m_cfg.verbosity != "info") {
        // If the user specified a verbosity we override the actual one (`info`)
        spdlog::set_level(spdlog::level::from_str(m_cfg.verbosity));
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

    m_mgr = std::make_unique<matcher_manager>(m_cfg);

    try {
        m_threads_table = t.get_table(THREAD_TABLE_NAME, st::SS_PLUGIN_ST_INT64);

        // pidns_init_start_ts used by TYPE_CONTAINER_START_TS and TYPE_CONTAINER_DURATION extractors
        m_threads_field_pidns_init_start_ts = m_threads_table.get_field(
                t.fields(), PIDNS_INIT_START_TS_FIELD_NAME, st::SS_PLUGIN_ST_UINT64);

        // vpid and ptid are used to attach the category field to the thread entry
        m_threads_field_vpid = m_threads_table.get_field(
                t.fields(), VPID_FIELD_NAME, st::SS_PLUGIN_ST_INT64);
        m_threads_field_ptid = m_threads_table.get_field(
                t.fields(), PTID_FIELD_NAME, st::SS_PLUGIN_ST_INT64);

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

        // Add the category field into thread table
        m_threads_field_category = m_threads_table.add_field(
                t.fields(), CATEGORY_FIELD_NAME, st::SS_PLUGIN_ST_UINT16);

    } catch(falcosecurity::plugin_exception e) {
        m_lasterr = "cannot add the '" + std::string(CONTAINER_ID_FIELD_NAME) +
                    "' field into the '" + std::string(THREAD_TABLE_NAME) +
                    "' table: " + e.what();
        SPDLOG_CRITICAL(m_lasterr);
        return false;
    }

    // Initialize dummy "host" fake container entry
    m_containers[HOST_CONTAINER_ID] = container_info::host_container_info();


    // Initialize metrics
    falcosecurity::metric n_container(METRIC_N_CONTAINERS);
    n_container.set_value(0);
    m_metrics.push_back(n_container);

    falcosecurity::metric n_missing(METRIC_N_MISSING);
    n_missing.set_value(0);
    m_metrics.push_back(n_missing);

    return true;
}

const std::vector<falcosecurity::metric>& my_plugin::get_metrics() {
    return m_metrics;
}

FALCOSECURITY_PLUGIN(my_plugin);