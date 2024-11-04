#pragma once

#include <cassert>
#include <cstdint>
#include <map>
#include <memory>
#include <list>
#include <string>
#include <vector>
#include "container_type.h"
#include <json/json.h>
#include <nlohmann/json.hpp>

class container_info {
public:
    class container_port_mapping {
    public:
        container_port_mapping(): m_host_ip(0), m_host_port(0), m_container_port(0) {}
        uint32_t m_host_ip;
        uint16_t m_host_port;
        uint16_t m_container_port;
    };

    class container_mount_info {
    public:
        container_mount_info():
                m_source(""),
                m_dest(""),
                m_mode(""),
                m_rdwr(false),
                m_propagation("") {}

        container_mount_info(const std::string &&source,
                             const std::string &&dest,
                             const std::string &&mode,
                             const bool rw,
                             const std::string &&propagation):
                m_source(source),
                m_dest(dest),
                m_mode(mode),
                m_rdwr(rw),
                m_propagation(propagation) {}

        container_mount_info(const Json::Value &source,
                             const Json::Value &dest,
                             const Json::Value &mode,
                             const Json::Value &rw,
                             const Json::Value &propagation) {
            get_string_value(source, m_source);
            get_string_value(dest, m_dest);
            get_string_value(mode, m_mode);
            get_string_value(propagation, m_propagation);

            if(!rw.isNull() && rw.isBool()) {
                m_rdwr = rw.asBool();
            }
        }

        std::string to_string() const {
            return m_source + ":" + m_dest + ":" + m_mode + ":" + (m_rdwr ? "true" : "false") +
                   ":" + m_propagation;
        }

        inline void get_string_value(const Json::Value &val, std::string &result) {
            if(!val.isNull() && val.isString()) {
                result = val.asString();
            }
        }

        std::string m_source;
        std::string m_dest;
        std::string m_mode;
        bool m_rdwr;
        std::string m_propagation;
    };

    class container_health_probe {
    public:
        // The type of health probe
        enum probe_type {
            PT_NONE = 0,
            PT_HEALTHCHECK,
            PT_LIVENESS_PROBE,
            PT_READINESS_PROBE,
            PT_END
        };

        // String representations of the above, suitable for
        // parsing to/from json. Should be kept in sync with
        // probe_type enum.
        static std::vector<std::string> probe_type_names;

        // Parse any health probes out of the provided
        // container json, updating the list of probes.
        static void parse_health_probes(const Json::Value &config_obj,
                                        std::list<container_health_probe> &probes);

        // Serialize the list of health probes, adding to the provided json object
        static void add_health_probes(const std::list<container_health_probe> &probes,
                                      Json::Value &config_obj);

        container_health_probe();
        container_health_probe(const probe_type probe_type,
                               const std::string &&exe,
                               const std::vector<std::string> &&args);
        virtual ~container_health_probe();

        // The probe_type that should be used for commands
        // matching this health probe.
        probe_type m_probe_type;

        // The actual health probe exe and args.
        std::string m_health_probe_exe;
        std::vector<std::string> m_health_probe_args;
    };

    container_info():
            m_type(CT_UNKNOWN),
            m_container_ip(0),
            m_privileged(false),
            m_host_pid(false),
            m_host_network(false),
            m_host_ipc(false),
            m_memory_limit(0),
            m_swap_limit(0),
            m_cpu_shares(1024),
            m_cpu_quota(0),
            m_cpu_period(100000),
            m_cpuset_cpu_count(0),
            m_is_pod_sandbox(false),
            m_container_user("<NA>"),
            m_metadata_deadline(0),
            m_size_rw_bytes(-1) {}

    void clear() {
        this->~container_info();
        new(this) container_info();
    }

    const std::vector<std::string> &get_env() const { return m_env; }

    const container_mount_info *mount_by_idx(uint32_t idx) const;
    const container_mount_info *mount_by_source(const std::string &) const;
    const container_mount_info *mount_by_dest(const std::string &) const;

    bool is_pod_sandbox() const { return m_is_pod_sandbox; }

    // static utilities to build a container_info
    static container_info host_container_info() {
        auto host_info = container_info();
        host_info.m_id = "host";
        host_info.m_full_id = "host";
        host_info.m_name = "host";
        return host_info;
    }

    static container_info from_json(nlohmann::json &root) {
        auto info = container_info();
        // TODO implement logic.
        // See https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/parsers.cpp#L4672
        return info;
    }

    std::string to_json() {
        nlohmann::json j;
        j["id"] = m_id;
        j["type"] = m_type;
        if (!m_full_id.empty()) {
            j["full_id"] = m_full_id;
        }
        if (!m_image.empty()) {
            j["image"] = m_image;
        }
        if (!m_name.empty()) {
            j["name"] = m_name;
        }
        if (!m_imagerepo.empty()) {
            j["imagerepo"] = m_imagerepo;
        }
        if (!m_imagetag.empty()) {
            j["imagetag"] = m_imagetag;
        }
        if (!m_imagedigest.empty()) {
            j["imagedigest"] = m_imagedigest;
        }
        nlohmann::json root;
        root["container"] = j;
        return root.dump();
    }

    // Match a process against the set of health probes
   // container_health_probe::probe_type match_health_probe(sinsp_threadinfo *tinfo) const;

    std::string m_id;
    std::string m_full_id;
    container_type m_type;
    std::string m_name;
    std::string m_image;
    std::string m_imageid;
    std::string m_imagerepo;
    std::string m_imagetag;
    std::string m_imagedigest;
    uint32_t m_container_ip;
    bool m_privileged;
    bool m_host_pid;
    bool m_host_network;
    bool m_host_ipc;
    std::vector<container_mount_info> m_mounts;
    std::vector<container_port_mapping> m_port_mappings;
    std::map<std::string, std::string> m_labels;
    std::vector<std::string> m_env;
    std::string m_mesos_task_id;
    int64_t m_memory_limit;
    int64_t m_swap_limit;
    int64_t m_cpu_shares;
    int64_t m_cpu_quota;
    int64_t m_cpu_period;
    int32_t m_cpuset_cpu_count;
    std::list<container_health_probe> m_health_probes;
    std::string m_pod_sandbox_id;
    std::map<std::string, std::string> m_pod_sandbox_labels;
    std::string m_pod_sandbox_cniresult;

    bool m_is_pod_sandbox;

    std::string m_container_user;

    uint64_t m_metadata_deadline;

    /**
     * The size of files that have been created or changed by this container.
     * This is not filled by default.
     */
    int64_t m_size_rw_bytes;

    /**
     * The time at which the container was created (IN SECONDS), cast from a value of `time_t`
     * We choose int64_t as we are not certain what type `time_t` is in a given
     * implementation; int64_t is the safest bet. Many default to int64_t anyway (e.g. CRI).
     */
    int64_t m_created_time;

    /**
     * The max container label length value. This is static because it is
     * universal across all instances and needs to be set once only.
     */
    static uint32_t m_container_label_max_length;
};