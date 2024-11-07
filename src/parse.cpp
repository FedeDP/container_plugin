#include "plugin.h"

//////////////////////////
// Parse capability
//////////////////////////

using nlohmann::json;

struct sinsp_param {
    uint16_t param_len;
    uint8_t* param_pointer;
};

/*
{
  "container": {
    "Mounts": [
      {
        "Destination": "/home/federico",
        "Mode": "",
        "Propagation": "rprivate",
        "RW": true,
        "Source": "/home/federico"
      }
    ],
    "User": "",
    "cni_json": "",
    "cpu_period": 100000,
    "cpu_quota": 0,
    "cpu_shares": 1024,
    "cpuset_cpu_count": 0,
    "created_time": 1730971086,
    "env": [],
    "full_id": "32a1026ccb88a551e2a38eb8f260b4700aefec7e8c007344057e58a9fa302374",
    "host_ipc": false,
    "host_network": false,
    "host_pid": false,
    "id": "32a1026ccb88",
    "image": "fedora:38",
    "imagedigest": "sha256:b9ff6f23cceb5bde20bb1f79b492b98d71ef7a7ae518ca1b15b26661a11e6a94",
    "imageid": "0ca0fed353fb77c247abada85aebc667fd1f5fa0b5f6ab1efb26867ba18f2f0a",
    "imagerepo": "fedora",
    "imagetag": "38",
    "ip": "172.17.0.2",
    "is_pod_sandbox": false,
    "labels": {
      "maintainer": "Clement Verna <cverna@fedoraproject.org>"
    },
    "lookup_state": 1,
    "memory_limit": 0,
    "metadata_deadline": 0,
    "name": "youthful_babbage",
    "pod_sandbox_id": "",
    "pod_sandbox_labels": null,
    "port_mappings": [],
    "privileged": false,
    "swap_limit": 0,
    "type": 0
  }
}
*/

void from_json(const json& j, container_health_probe& probe) {
    probe.m_args = j.value("args", std::vector<std::string>{});
    probe.m_exe = j.value("exe", "");
}

void from_json(const json& j, container_mount_info& mount) {
    mount.m_source = j.value("Source", "");
    mount.m_dest = j.value("Destination", "");
    mount.m_mode = j.value("Mode", "");
    mount.m_rdwr = j.value("RW", false);
    mount.m_propagation = j.value("Propagation", "");
}

void from_json(const json& j, container_port_mapping& port) {
    port.m_host_ip = j.value("HostIp", 0);
    port.m_host_port = j.value("HostPort", 0);
    port.m_container_port = j.value("ContainerPort", 0);
}

void from_json(const json& j, std::shared_ptr<container_info>& cinfo) {
    const json& container = j["container"];
    cinfo->m_type = container.value("type", CT_UNKNOWN);
    cinfo->m_id = container.value("id", "");
    cinfo->m_name = container.value("name", "");
    cinfo->m_image = container.value("image", "");
    cinfo->m_imagedigest = container.value("imagedigest", "");
    cinfo->m_imageid = container.value("imageid", "");
    cinfo->m_imagerepo = container.value("imagerepo", "");
    cinfo->m_imagetag = container.value("imagetag", "");
    cinfo->m_container_user = container.value("User", "");
    cinfo->m_pod_sandbox_cniresult = container.value("cni_json", "");
    cinfo->m_cpu_period = container.value("cpu_period", 0);
    cinfo->m_cpu_quota = container.value("cpu_quota", 0);
    cinfo->m_cpu_shares = container.value("cpu_shares", 0);
    cinfo->m_cpuset_cpu_count = container.value("cpuset_cpu_count", 0);
    cinfo->m_created_time = container.value("created_time", 0);
    cinfo->m_env = container.value("env", std::vector<std::string>{});
    cinfo->m_full_id = container.value("full_id", "");
    cinfo->m_host_ipc = container.value("host_ipc", false);
    cinfo->m_host_network = container.value("host_network", false);
    cinfo->m_host_pid = container.value("host_pid", false);
    cinfo->m_container_ip = container.value("ip", 0);
    cinfo->m_is_pod_sandbox = container.value("is_pod_sandbox", false);
    cinfo->m_labels = container.value("labels", std::map<std::string, std::string>{});
    cinfo->m_memory_limit = container.value("memory_limit", 0);
    cinfo->m_swap_limit = container.value("swap_limit", 0);
    cinfo->m_pod_sandbox_id = container.value("pod_sandbox_id", "");
    cinfo->m_privileged = container.value("privileged", false);
    cinfo->m_pod_sandbox_labels = container.value("pod_sandbox_labels", std::map<std::string, std::string>{});
    cinfo->m_port_mappings = container.value("port_mappings", std::vector<container_port_mapping>{});
    cinfo->m_mounts =  container.value("Mounts", std::vector<container_mount_info>{});

    for (int probe_type = container_health_probe::PT_HEALTHCHECK; probe_type <= container_health_probe::PT_READINESS_PROBE; probe_type++) {
        const auto& probe_name = container_health_probe::probe_type_names[probe_type];
        if (container.contains(probe_name)) {
            container_health_probe probe = container.value(probe_name, container_health_probe());
            probe.m_type = container_health_probe::probe_type(probe_type);
            cinfo->m_health_probes.push_back(probe);
        }
    }
}

void to_json(json& j, const container_mount_info& mount) {
    j["Source"] = mount.m_source;
    j["Destination"] = mount.m_dest;
    j["Mode"] = mount.m_mode;
    j["RW"] = mount.m_rdwr;
    j["Propagation"] = mount.m_propagation;
}

void to_json(json& j, const container_port_mapping& port) {
    j["HostIp"] = port.m_host_ip;
    j["HostPort"] = port.m_host_port;
    j["ContainerPort"] = port.m_container_port;
}

void to_json(json& j, const std::shared_ptr<container_info>& cinfo) {
    auto& container = j["container"];
    j["type"] = cinfo->m_type;
    j["id"] = cinfo->m_id;
    j["name"] = cinfo->m_name;
    j["image"] = cinfo->m_image;
    j["imagedigest"] = cinfo->m_imagedigest;
    j["imageid"] = cinfo->m_imageid;
    j["imagerepo"] = cinfo->m_imagerepo;
    j["imagetag"] = cinfo->m_imagetag;
    j["User"] = cinfo->m_container_user;
    j["cni_json"] = cinfo->m_pod_sandbox_cniresult;
    j["cpu_period"] = cinfo->m_cpu_period;
    j["cpu_quota"] = cinfo->m_cpu_quota;
    j["cpu_shares"] = cinfo->m_cpu_shares;
    j["cpuset_cpu_count"] = cinfo->m_cpuset_cpu_count;
    j["created_time"] = cinfo->m_created_time;
    // TODO: only append a limited set of env?
    // https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L232
    j["env"] = cinfo->m_env;
    j["full_id"] = cinfo->m_full_id;
    j["host_ipc"] = cinfo->m_host_ipc;
    j["host_network"] = cinfo->m_host_network;
    j["host_pid"] = cinfo->m_host_pid;
    j["ip"] = cinfo->m_container_ip;
    j["is_pod_sandbox"] = cinfo->m_is_pod_sandbox;
    j["labels"] = cinfo->m_labels;
    j["memory_limit"] = cinfo->m_memory_limit;
    j["swap_limit"] = cinfo->m_swap_limit;
    j["pod_sandbox_id"] = cinfo->m_pod_sandbox_id;
    j["privileged"] = cinfo->m_privileged;
    j["pod_sandbox_labels"] = cinfo->m_pod_sandbox_labels;
    j["port_mappings"] = cinfo->m_port_mappings;
    j["Mounts"] = cinfo->m_mounts;

    for(auto &probe : cinfo->m_health_probes) {
        const auto probe_type = container_health_probe::probe_type_names[probe.m_type];
        j[probe_type]["exe"] = probe.m_exe;
        auto args = json::array();
        for(auto &arg : probe.m_args) {
            args.push_back(arg);
        }
        j[probe_type]["args"] = args;
    }
}

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


// We need to parse only the async events produced by this plugin. The async
// events produced by this plugin are injected in the syscall event source,
// so here we need to parse events coming from the "syscall" source.
// We will select specific events to parse through the
// `get_parse_event_types` API.
std::vector<std::string> my_plugin::get_parse_event_sources() {
    return PARSE_EVENT_SOURCES;
}

std::vector<falcosecurity::event_type> my_plugin::get_parse_event_types() {
    return PARSE_EVENT_CODES;
}

bool my_plugin::parse_async_event(
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

    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    if (added) {
        m_containers[cinfo->m_id] = cinfo;
    } else {
        m_containers.erase(cinfo->m_id);
    }

    // Update n_containers metric
    m_metrics.at(0).set_value(m_containers.size() - 1);

    // Update n_missing metric
    auto val = m_metrics.at(1).value.u64;
    if (!cinfo->m_is_pod_sandbox && cinfo->m_image.empty()) {
        m_metrics.at(1).set_value(val + 1);
    }
    return true;
}

bool my_plugin::parse_container_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    auto id_param = get_syscall_evt_param(evt.get_buf(), 0);
    auto type_param = get_syscall_evt_param(evt.get_buf(), 1);
    auto name_param = get_syscall_evt_param(evt.get_buf(), 2);
    auto image_param = get_syscall_evt_param(evt.get_buf(), 3);

    std::string id = (char*)id_param.param_pointer;
    container_type tp = *((container_type*)type_param.param_pointer);
    std::string name = (char*)name_param.param_pointer;
    std::string image = (char*)image_param.param_pointer;

    auto cinfo = std::make_shared<container_info>();
    cinfo->m_id = id;
    cinfo->m_type = tp;
    cinfo->m_name = name;
    cinfo->m_image = image;
    m_containers[id] = cinfo;
    return true;
}

bool my_plugin::parse_container_json_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    auto json_param = get_syscall_evt_param(evt.get_buf(), 0);

    std::string json_str = (char *)json_param.param_pointer;
    auto json_event = nlohmann::json::parse(json_str);

    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    m_containers[cinfo->m_id] = cinfo;
    return true;
}


std::string my_plugin::compute_container_id_for_thread(const falcosecurity::table_entry& thread_entry,
                                                       const falcosecurity::table_reader& tr,
                                                       std::shared_ptr<container_info>& info) {
    // retrieve tid cgroups, compute container_id and store it.
    std::string container_id;
    using st = falcosecurity::state_value_type;

    // get the cgroups table of the thread
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
                    if (m_mgr->match_cgroup(cgroup, container_id, info)) {
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

bool my_plugin::parse_new_process_event(
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

    // retrieve the thread entry associated with this thread id
    auto thread_entry = m_threads_table.get_entry(tr, thread_id);

    std::shared_ptr<container_info> info = nullptr;
    auto container_id = compute_container_id_for_thread(thread_entry, tr, info);

    // store container_id
    auto& tw = in.get_table_writer();
    m_container_id_field.write_value(tw, thread_entry,
                                     (const char*)container_id.c_str());

    if (info != nullptr) {
        // Since the matcher also returned a container_info,
        // it means we do not expect to receive any metadata from the go-worker,
        // since the engine has no listener SDK.
        // Just send the event now.
        nlohmann::json j(info);
        generate_async_event(j.dump().c_str(), true);
    }
    return true;
}

bool my_plugin::parse_event(const falcosecurity::parse_event_input& in) {
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
        case PPME_SYSCALL_EXECVE_16_X:
        case PPME_SYSCALL_EXECVE_17_X:
        case PPME_SYSCALL_EXECVE_18_X:
        case PPME_SYSCALL_EXECVE_19_X:
        case PPME_SYSCALL_EXECVEAT_X:
        case PPME_SYSCALL_CHROOT_X:
            return parse_new_process_event(in);
        default:
            SPDLOG_ERROR("received an unknown event type {}",
                         int32_t(evt.get_type()));
            return false;
    }
}

FALCOSECURITY_PLUGIN_EVENT_PARSING(my_plugin);