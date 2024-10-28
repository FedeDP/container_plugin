// SPDX-License-Identifier: Apache-2.0
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

#include <utility>
#include <re2/re2.h>
#include "container_info.h"

std::vector<std::string> container_info::container_health_probe::probe_type_names =
        {"None", "Healthcheck", "LivenessProbe", "ReadinessProbe", "End"};

// Initialize container max label length to default 100 value
uint32_t container_info::m_container_label_max_length = 100;

container_info::container_health_probe::container_health_probe() {}

container_info::container_health_probe::container_health_probe(
        const probe_type ptype,
        const std::string &&exe,
        const std::vector<std::string> &&args):
        m_probe_type(ptype),
        m_health_probe_exe(exe),
        m_health_probe_args(args) {}

container_info::container_health_probe::~container_health_probe() {}

void container_info::container_health_probe::parse_health_probes(
        const Json::Value &config_obj,
        std::list<container_health_probe> &probes) {
    // Add any health checks described in the container config/labels.
    for(int i = PT_NONE; i != PT_END; i++) {
        std::string key = probe_type_names[i];
        const Json::Value &probe_obj = config_obj[key];

        if(!probe_obj.isNull() && probe_obj.isObject()) {
            const Json::Value &probe_exe_obj = probe_obj["exe"];

            if(!probe_exe_obj.isNull() && probe_exe_obj.isConvertibleTo(Json::stringValue)) {
                const Json::Value &probe_args_obj = probe_obj["args"];

                std::string probe_exe = probe_exe_obj.asString();
                std::vector<std::string> probe_args;

                if(!probe_args_obj.isNull() && probe_args_obj.isArray()) {
                    for(const auto &item : probe_args_obj) {
                        if(item.isConvertibleTo(Json::stringValue)) {
                            probe_args.push_back(item.asString());
                        }
                    }
                }

                probes.emplace_back(static_cast<probe_type>(i),
                                    std::move(probe_exe),
                                    std::move(probe_args));
            }
        }
    }
}

void container_info::container_health_probe::add_health_probes(
        const std::list<container_health_probe> &probes,
        Json::Value &config_obj) {
    for(auto &probe : probes) {
        std::string key = probe_type_names[probe.m_probe_type];
        Json::Value args;

        config_obj[key]["exe"] = probe.m_health_probe_exe;
        for(auto &arg : probe.m_health_probe_args) {
            args.append(arg);
        }

        config_obj[key]["args"] = args;
    }
}

const container_info::container_mount_info *container_info::mount_by_idx(
        uint32_t idx) const {
    if(idx >= m_mounts.size()) {
        return NULL;
    }

    return &(m_mounts[idx]);
}

const container_info::container_mount_info *container_info::mount_by_source(
        const std::string &source) const {
    // note: linear search
    re2::RE2 pattern(source, re2::RE2::POSIX);
    for(auto &mntinfo : m_mounts) {
        if(re2::RE2::PartialMatch(mntinfo.m_source.c_str(), pattern)) {
            return &mntinfo;
        }
    }

    return NULL;
}

const container_info::container_mount_info *container_info::mount_by_dest(
        const std::string &dest) const {
    // note: linear search
    re2::RE2 pattern(dest, re2::RE2::POSIX);
    for(auto &mntinfo : m_mounts) {
        if(re2::RE2::PartialMatch(mntinfo.m_dest.c_str(), pattern)) {
            return &mntinfo;
        }
    }

    return NULL;
}

// TODO reimplement in go
/*container_info::container_health_probe::probe_type container_info::match_health_probe(
        sinsp_threadinfo *tinfo) const {

    auto pred = [&](const container_health_probe &p) {
        return (p.m_health_probe_exe == tinfo->m_exe && p.m_health_probe_args == tinfo->m_args);
    };

    auto match = std::find_if(m_health_probes.begin(), m_health_probes.end(), pred);

    if(match == m_health_probes.end()) {
        return container_health_probe::PT_NONE;
    }

    return match->m_probe_type;
}*/