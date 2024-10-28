#include "plugin.h"
#include <arpa/inet.h>

//////////////////////////
// Extract capability
//////////////////////////

// Keep this aligned with `get_fields`
enum ContainerFields {
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

std::vector<std::string> my_plugin::get_extract_event_sources() {
    return EXTRACT_EVENT_SOURCES;
}

std::vector<falcosecurity::field_info> my_plugin::get_fields() {
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
            {ft::FTYPE_STRING, "container.image", "Image Name",
                    "The container image name (e.g. falcosecurity/falco:latest for docker). In instances of "
                    "userspace container engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.image.id", "Image ID",
                    "The container image id (e.g. 6f7e2741b66b). In instances of userspace container engine "
                    "lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.type", "Type",
                    "The container type, e.g. docker, cri-o, containerd etc."},
            {ft::FTYPE_BOOL, "container.privileged", "Privileged",
                    "'true' for containers running as privileged, 'false' otherwise. In instances of "
                    "userspace container engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.mounts", "Mounts",
                    "A space-separated list of mount information. Each item in the list has the format "
                    "'source:dest:mode:rdrw:propagation'. In instances of userspace container engine lookup "
                    "delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.mount", "Mount",
                    "Information about a single mount, specified by number (e.g. container.mount[0]) or mount "
                    "source (container.mount[/usr/local]). The pathname can be a glob "
                    "(container.mount[/usr/local/*]), in which case the first matching mount will be "
                    "returned. The information has the format 'source:dest:mode:rdrw:propagation'. If there "
                    "is no mount with the specified index or matching the provided source, returns the string "
                    "\"none\" instead of a NULL value. In instances of userspace container engine lookup "
                    "delays, this field may not be available yet.", falcosecurity::field_arg()},
            {ft::FTYPE_STRING, "container.mount.source", "Mount Source",
                    "The mount source, specified by number (e.g. container.mount.source[0]) or mount "
                    "destination (container.mount.source[/host/lib/modules]). The pathname can be a glob. In "
                    "instances of userspace container engine lookup delays, this field may not be available "
                    "yet.", falcosecurity::field_arg()},
            {ft::FTYPE_STRING, "container.mount.dest", "Mount Destination",
                    "The mount destination, specified by number (e.g. container.mount.dest[0]) or mount "
                    "source (container.mount.dest[/lib/modules]). The pathname can be a glob. In instances of "
                    "userspace container engine lookup delays, this field may not be available yet.",
                    falcosecurity::field_arg()},
            {ft::FTYPE_STRING, "container.mount.mode", "Mount Mode",
                    "The mount mode, specified by number (e.g. container.mount.mode[0]) or mount source "
                    "(container.mount.mode[/usr/local]). The pathname can be a glob. In instances of "
                    "userspace container engine lookup delays, this field may not be available yet.",
                    falcosecurity::field_arg()},
            {ft::FTYPE_STRING, "container.mount.rdwr", "Mount Read/Write",
                    "The mount rdwr value, specified by number (e.g. container.mount.rdwr[0]) or mount source "
                    "(container.mount.rdwr[/usr/local]). The pathname can be a glob. In instances of "
                    "userspace container engine lookup delays, this field may not be available yet.",
                    falcosecurity::field_arg()},
            {ft::FTYPE_STRING, "container.mount.propagation", "Mount Propagation",
                    "The mount propagation value, specified by number (e.g. container.mount.propagation[0]) "
                    "or mount source (container.mount.propagation[/usr/local]). The pathname can be a glob. "
                    "In instances of userspace container engine lookup delays, this field may not be "
                    "available yet.", falcosecurity::field_arg()},
            {ft::FTYPE_STRING, "container.image.repository", "Repository",
                    "The container image repository (e.g. falcosecurity/falco). In instances of userspace "
                    "container engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.image.tag", "Image Tag",
                    "The container image tag (e.g. stable, latest). In instances of userspace container "
                    "engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.image.digest", "Registry Digest",
                    "The container image registry digest (e.g. "
                    "sha256:d977378f890d445c15e51795296e4e5062f109ce6da83e0a355fc4ad8699d27). In instances of "
                    "userspace container engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.healthcheck", "Health Check",
                    "The container's health check. Will be the null value (\"N/A\") if no healthcheck "
                    "configured, \"NONE\" if configured but explicitly not created, and the healthcheck "
                    "command line otherwise. In instances of userspace container engine lookup delays, this "
                    "field may not be available yet."},
            {ft::FTYPE_STRING, "container.liveness_probe", "Liveness",
                    "The container's liveness probe. Will be the null value (\"N/A\") if no liveness probe "
                    "configured, the liveness probe command line otherwise. In instances of userspace "
                    "container engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.readiness_probe", "Readiness",
                    "The container's readiness probe. Will be the null value (\"N/A\") if no readiness probe "
                    "configured, the readiness probe command line otherwise. In instances of userspace "
                    "container engine lookup delays, this field may not be available yet."},
            {ft::FTYPE_ABSTIME, "container.start_ts", "Container Start",
                    "Container start as epoch timestamp in nanoseconds based on proc.pidns_init_start_ts and "
                    "extracted in the kernel and not from the container runtime socket / container engine."},
            {ft::FTYPE_RELTIME, "container.duration", "Container Duration",
                    "Number of nanoseconds since container.start_ts."},
            {ft::FTYPE_STRING, "container.ip", "Container ip address",
                    "The container's / pod's primary ip address as retrieved from the container engine. Only "
                    "ipv4 addresses are tracked. Consider container.cni.json (CRI use case) for logging ip "
                    "addresses for each network interface. In instances of userspace container engine lookup "
                    "delays, this field may not be available yet."},
            {ft::FTYPE_STRING, "container.cni.json", "Container's / pod's CNI result json",
                    "The container's / pod's CNI result field from the respective pod status info. It "
                    "contains ip addresses for each network interface exposed as unparsed escaped JSON "
                    "string. Supported for CRI container engine (containerd, cri-o runtimes), optimized for "
                    "containerd (some non-critical JSON keys removed). Useful for tracking ips (ipv4 and "
                    "ipv6, dual-stack support) for each network interface (multi-interface support). In "
                    "instances of userspace container engine lookup delays, this field may not be available "
                    "yet."},
            {ft::FTYPE_BOOL, "container.host_pid", "Host PID Namespace",
                    "'true' if the container is running in the host PID namespace, 'false' otherwise."},
            {ft::FTYPE_BOOL, "container.host_network", "Host Network Namespace",
                    "'true' if the container is running in the host network namespace, 'false' otherwise."},
            {ft::FTYPE_BOOL, "container.host_ipc", "Host IPC Namespace",
                    "'true' if the container is running in the host IPC namespace, 'false' otherwise."},
    };
    const int fields_size = sizeof(fields) / sizeof(fields[0]);
    static_assert(fields_size == TYPE_CONTAINER_FIELD_MAX, "Wrong number of container fields.");
    return std::vector<falcosecurity::field_info>(fields, fields + fields_size);
}

bool my_plugin::extract(const falcosecurity::extract_fields_input& in) {
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
    const auto field_id = req.get_field_id();
    switch(field_id)
    {
        case TYPE_CONTAINER_ID:
            req.set_value(container_info.m_id);
            break;
        case TYPE_CONTAINER_FULL_CONTAINER_ID:
            req.set_value(container_info.m_full_id);
            break;
        case TYPE_CONTAINER_NAME:
            req.set_value(container_info.m_name);
            break;
        case TYPE_CONTAINER_IMAGE:
            req.set_value(container_info.m_image);
            break;
        case TYPE_CONTAINER_IMAGE_ID:
            req.set_value(container_info.m_imageid);
            break;
        case TYPE_CONTAINER_TYPE:
            req.set_value(to_string(container_info.m_type));
            break;
        case TYPE_CONTAINER_PRIVILEGED:
            req.set_value(container_info.m_privileged);
            break;
        case TYPE_CONTAINER_MOUNTS: {
            std::string tstr;
            bool first = true;
            for (auto &mntinfo: container_info.m_mounts) {
                if (first) {
                    first = false;
                } else {
                    tstr += ",";
                }
                tstr += mntinfo.to_string();
            }
            req.set_value(tstr);
            break;
        }
        case TYPE_CONTAINER_MOUNT:
        case TYPE_CONTAINER_MOUNT_SOURCE:
        case TYPE_CONTAINER_MOUNT_DEST:
        case TYPE_CONTAINER_MOUNT_MODE:
        case TYPE_CONTAINER_MOUNT_RDWR:
        case TYPE_CONTAINER_MOUNT_PROPAGATION: {
            const sinsp_container_info::container_mount_info *mntinfo;
            auto arg_id = req.get_arg_index();
            if (arg_id != -1) {
                mntinfo = container_info.mount_by_idx(arg_id);
            } else {
                auto arg_key = req.get_arg_key();
                mntinfo = container_info.mount_by_source(arg_key);
            }
            if (mntinfo) {
                std::string tstr;
                switch (field_id) {
                case TYPE_CONTAINER_MOUNT:
                    tstr = mntinfo->to_string();
                    break;
                case TYPE_CONTAINER_MOUNT_SOURCE:
                    tstr = mntinfo->m_source;
                    break;
                case TYPE_CONTAINER_MOUNT_DEST:
                    tstr = mntinfo->m_dest;
                    break;
                case TYPE_CONTAINER_MOUNT_MODE:
                    tstr = mntinfo->m_mode;
                    break;
                case TYPE_CONTAINER_MOUNT_RDWR:
                    tstr = (mntinfo->m_rdwr ? "true" : "false");
                    break;
                case TYPE_CONTAINER_MOUNT_PROPAGATION:
                    tstr = mntinfo->m_propagation;
                    break;
                }
                req.set_value(tstr);
            }
            break;
        }
        case TYPE_CONTAINER_IMAGE_REPOSITORY:
            req.set_value(container_info.m_imagerepo);
            break;
        case TYPE_CONTAINER_IMAGE_TAG:
            req.set_value(container_info.m_imagetag);
            break;
        case TYPE_CONTAINER_IMAGE_DIGEST:
            req.set_value(container_info.m_imagedigest);
            break;
        case TYPE_CONTAINER_HEALTHCHECK:
        case TYPE_CONTAINER_LIVENESS_PROBE:
        case TYPE_CONTAINER_READINESS_PROBE: {
            std::string tstr = "NONE";
            bool set = false;
            for(auto &probe : container_info.m_health_probes) {
                if((field_id == TYPE_CONTAINER_HEALTHCHECK &&
                    probe.m_probe_type ==
                    sinsp_container_info::container_health_probe::PT_HEALTHCHECK) ||
                   (field_id == TYPE_CONTAINER_LIVENESS_PROBE &&
                    probe.m_probe_type ==
                    sinsp_container_info::container_health_probe::PT_LIVENESS_PROBE) ||
                   (field_id == TYPE_CONTAINER_READINESS_PROBE &&
                    probe.m_probe_type ==
                    sinsp_container_info::container_health_probe::PT_READINESS_PROBE)) {
                    tstr = probe.m_health_probe_exe;

                    for(auto &arg : probe.m_health_probe_args) {
                        tstr += " ";
                        tstr += arg;
                    }
                    req.set_value(tstr);
                    set = true;
                    break;
                }
            }
            if (!set) {
                req.set_value(tstr);
            }
            break;
        }
        case TYPE_CONTAINER_START_TS:
            // TODO...uses tinfo :/
            break;
        case TYPE_CONTAINER_DURATION:
            // TODO...uses tinfo :/
            break;
        case TYPE_CONTAINER_IP_ADDR: {
            uint32_t
            val = htonl(container_info.m_container_ip);
            char addrbuff[100];
            inet_ntop(AF_INET, &val, addrbuff, sizeof(addrbuff));
            req.set_value(addrbuff);
            break;
        }
        case TYPE_CONTAINER_CNIRESULT:
            req.set_value(container_info.m_pod_sandbox_cniresult);
            break;
        case TYPE_CONTAINER_HOST_PID:
            req.set_value(container_info.m_host_pid);
            break;
        case TYPE_CONTAINER_HOST_NETWORK:
            req.set_value(container_info.m_host_network);
            break;
        case TYPE_CONTAINER_HOST_IPC:
            req.set_value(container_info.m_host_ipc);
            break;
        default:
            SPDLOG_ERROR(
                    "unknown extraction request on field '{}' for container_id '{}'",
                    req.get_field_id(), container_id);
            return false;
    }
    return true;
}

FALCOSECURITY_PLUGIN_FIELD_EXTRACTION(my_plugin);