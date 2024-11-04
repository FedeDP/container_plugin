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
#include <filesystem>

namespace fs = std::filesystem;

using nlohmann::json;

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
		},
        "engines": {
            "$ref": "#/definitions/Engines",
            "title": "The plugin per-engine configuration",
			"description": "Allows to disable/enable each engine and customize sockets where available."
        }
	},
    "definitions": {
        "Engines": {
            "type": "object",
            "additionalProperties": false,
            "properties": {
                "docker": {
                    "$ref": "#/definitions/SocketsContainer"
                },
                "podman": {
                    "$ref": "#/definitions/SocketsContainer"
                },
                "containerd": {
                    "$ref": "#/definitions/SocketsContainer"
                },
                "cri": {
                    "$ref": "#/definitions/SocketsContainer"
                },
                "lxc": {
                    "$ref": "#/definitions/SimpleContainer"
                },
                "libvirt_lxc": {
                    "$ref": "#/definitions/SimpleContainer"
                },
                "bpm": {
                    "$ref": "#/definitions/SimpleContainer"
                },
                "static": {
                    "$ref": "#/definitions/StaticContainer"
                }
            },
            "required": [
                "bpm",
                "containerd",
                "cri",
                "docker",
                "libvirt_lxc",
                "lxc",
                "podman"
            ],
            "title": "Engines"
        },
        "nonEmptyString": {
            "type": "string",
            "minLength": 1
        },
        "SimpleContainer": {
            "type": "object",
            "additionalProperties": false,
            "properties": {
                "enabled": {
                    "type": "boolean"
                }
            },
            "required": [
                "enabled"
            ],
            "title": "SimpleContainer"
        },
        "SocketsContainer": {
            "type": "object",
            "additionalProperties": false,
            "properties": {
                "enabled": {
                    "type": "boolean"
                },
                "sockets": {
                    "type": "array",
                    "items": {
                        "type": "string"
                    }
                }
            },
            "required": [
                "enabled",
                "sockets"
            ],
            "title": "SocketsContainer"
        },
        "StaticContainer": {
            "type": "object",
            "additionalProperties": false,
            "properties": {
                "enabled": {
                    "type": "boolean"
                },
                "container_id": {
                    "$ref": "#/definitions/nonEmptyString"
                },
                "container_name": {
                    "$ref": "#/definitions/nonEmptyString"
                },
                "container_image": {
                    "$ref": "#/definitions/nonEmptyString"
                }
            },
            "required": [
                "enabled",
                "container_id",
                "container_name",
                "container_image"
            ],
            "title": "StaticContainer"
        }
    },
	"additionalProperties": false,
	"type": "object"
})";
    return init_schema;
}

void from_json(const json& j, StaticEngine& engine) {
    engine.enabled = j.value("enabled", false);
    engine.name = j.value("container_name", "");
    engine.id = j.value("container_id", "");
    engine.image = j.value("container_image", "");
}

void from_json(const json& j, SimpleEngine& engine) {
    engine.enabled = j.value("enabled", true);
}

void from_json(const json& j, SocketsEngine& engine) {
    engine.enabled = j.value("enabled", true);
    engine.sockets = j.value("sockets", std::vector<std::string>{});
}

void from_json(const json& j, PluginConfig& cfg) {
    cfg.verbosity = j.value("verbosity", "info");
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
        for (const auto & entry : fs::directory_iterator("/run/user")) {
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

uint64_t my_plugin::get_container_engine_mask() {
    uint64_t container_mask = 0;
    if (m_cfg.containerd.enabled) {
        container_mask |= 1 << CT_CONTAINERD;
    }
    if (m_cfg.podman.enabled) {
        container_mask |= 1 << CT_PODMAN;
    }
    if (m_cfg.cri.enabled) {
        container_mask |= 1 << CT_CRI;
    }
    if (m_cfg.docker.enabled) {
        container_mask |= 1 << CT_DOCKER;
    }
    if (m_cfg.lxc.enabled) {
        container_mask |= 1 << CT_LXC;
    }
    if (m_cfg.libvirt_lxc.enabled) {
        container_mask |= 1 << CT_LIBVIRT_LXC;
    }
    if (m_cfg.bpm.enabled) {
        container_mask |= 1 << CT_BPM;
    }
    if (m_cfg.static_ctr.enabled) {
        container_mask |= 1 << CT_STATIC;
    }
    return container_mask;
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

    m_mgr = std::make_unique<matcher_manager>(get_container_engine_mask());

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