#include "plugin.h"

//////////////////////////
// Extract capability
//////////////////////////

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
    };
    const int fields_size = sizeof(fields) / sizeof(fields[0]);
    // TODO: uncomment once all fields are exposed.
    // static_assert(fields_size == TYPE_CONTAINER_FIELD_MAX, "Wrong number of container fields.");
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

FALCOSECURITY_PLUGIN_FIELD_EXTRACTION(my_plugin);