#include "plugin.h"

//////////////////////////
// Parse capability
//////////////////////////

struct sinsp_param {
    uint32_t param_len;
    uint8_t* param_pointer;
};

// Obtain a param from a sinsp event
template <const bool LargePayload=false, typename T=std::conditional_t<LargePayload, uint32_t*, uint16_t*>>
static inline sinsp_param get_syscall_evt_param(void* evt, uint32_t num_param)
{
    uint32_t dataoffset = 0;
    // pointer to the lengths array inside the event.
    auto len = (T)((uint8_t*)evt + sizeof(falcosecurity::_internal::ss_plugin_event));
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
    if(!added && !removed) {
        // We are not interested in parsing async events that are not
        // generated by our plugin.
        // This is not an error, it could happen when we have more than one
        // async plugin loaded.
        return true;
    }

    uint32_t json_charbuf_len = 0;
    char* json_charbuf_pointer = (char*)ad.get_data(json_charbuf_len);
    if(json_charbuf_pointer == nullptr) {
        m_lasterr = "there is no payload in the async event";
        SPDLOG_ERROR(m_lasterr);
        return false;
    }
    auto json_event = nlohmann::json::parse(json_charbuf_pointer);
    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    if (added) {
        SPDLOG_TRACE("Adding container: {}", cinfo->m_id);
        m_containers[cinfo->m_id] = cinfo;
        m_last_container = cinfo;
    } else {
        SPDLOG_TRACE("Removing container: {}", cinfo->m_id);
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
    SPDLOG_TRACE("Adding container from old container event: {}", cinfo->m_id);
    m_containers[id] = cinfo;
    m_last_container = cinfo;
    return true;
}

bool my_plugin::parse_container_json_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    auto json_param = get_syscall_evt_param(evt.get_buf(), 0);

    std::string json_str = (char *)json_param.param_pointer;
    auto json_event = nlohmann::json::parse(json_str);

    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    SPDLOG_TRACE("Adding container from old container_json event: {}", cinfo->m_id);
    m_containers[cinfo->m_id] = cinfo;
    m_last_container = cinfo;
    return true;
}

bool my_plugin::parse_container_json_2_event(
        const falcosecurity::parse_event_input& in) {
    auto& evt = in.get_event_reader();
    auto json_param = get_syscall_evt_param<true>(evt.get_buf(), 0);

    std::string json_str = (char *)json_param.param_pointer;
    auto json_event = nlohmann::json::parse(json_str);

    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    SPDLOG_TRACE("Adding container from old container_json_2 event: {}", cinfo->m_id);
    m_containers[cinfo->m_id] = cinfo;
    m_last_container = cinfo;
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
                    m_mgr->match_cgroup(cgroup, container_id, info);
                    if (!container_id.empty()) {
                        SPDLOG_DEBUG("Matched container_id: {} from cgroup {}", container_id, cgroup);
                      	// break the loop
                      	return false;
                    }
                }
                return true;
            }
    );
    return container_id;
}

// Same logic as https://github.com/falcosecurity/libs/blob/a99a36573f59c0e25965b36f8fa4ae1b10c5d45c/userspace/libsinsp/container.cpp#L438
void my_plugin::write_thread_category(const std::shared_ptr<const container_info>& cinfo,
    								  const falcosecurity::table_entry& thread_entry,
                                      const falcosecurity::table_reader& tr,
                                      const falcosecurity::table_writer& tw) {
    using st = falcosecurity::state_value_type;

    int64_t vpid;
    m_threads_field_vpid.read_value(tr, thread_entry, vpid);
    if (vpid == 1) {
        uint16_t category = CAT_CONTAINER;
        m_threads_field_category.write_value(tw, thread_entry, category);
        return;
    }

    int64_t ptid;
    m_threads_field_ptid.read_value(tr, thread_entry, ptid);
    try {
        auto parent_entry = m_threads_table.get_entry(tr, ptid);
        uint16_t parent_category;
        m_threads_field_category.read_value(tr, parent_entry, parent_category);
        if (parent_category != CAT_NONE) {
            m_threads_field_category.write_value(tw, thread_entry, parent_category);
            return;
        }
    } catch (falcosecurity::plugin_exception &ex) {
        // nothing
        SPDLOG_DEBUG("no parent thread found");
    }

    // Read "exe" field
    std::string exe;
    m_threads_field_exe.read_value(tr, thread_entry, exe);
    // Read "args" field: collect args
    std::vector<std::string> args;
    auto args_table = m_threads_table.get_subtable(
                    tr, m_threads_field_args, thread_entry,
                    st::SS_PLUGIN_ST_INT64);
    args_table.iterate_entries(
                    tr,
                    [this, tr, &args](const falcosecurity::table_entry& e)
                    {
                        // read the arg field from the current entry of args
                        // table
                        std::string arg;
                        m_args_field.read_value(tr, e, arg);
                        if(!arg.empty())
                        {
                            args.push_back(arg);
                        }
                        return true;
                    });

    const auto ptype = cinfo->match_health_probe(exe, args);
	if(ptype == container_health_probe::PT_NONE) {
		return;
	}

    bool found_container_init = false;
    while (!found_container_init) {
        try {
            // Move to parent
            auto entry = m_threads_table.get_entry(tr, ptid);

            // Read vpid and container_id for parent
            int64_t vpid;
            std::string container_id;
            m_threads_field_vpid.read_value(tr, entry, vpid);
            m_container_id_field.read_value(tr, entry, container_id);

            if (vpid == 1 && !container_id.empty()) {
                found_container_init = true;
            } else {
                // update ptid for next iteration
                m_threads_field_ptid.read_value(tr, entry, ptid);
            }
        } catch (falcosecurity::plugin_exception &ex) {
            // end of loop
            break;
        }
    }
    if (!found_container_init) {
        uint16_t category;
      	// Each health probe type maps to a command category
		switch(ptype) {
		case container_health_probe::PT_NONE:
			break;
		case container_health_probe::PT_HEALTHCHECK:
            category = CAT_HEALTHCHECK;
            m_threads_field_category.write_value(tw, thread_entry, category);
			break;
		case container_health_probe::PT_LIVENESS_PROBE:
            category = CAT_LIVENESS_PROBE;
            m_threads_field_category.write_value(tw, thread_entry, category);
			break;
		case container_health_probe::PT_READINESS_PROBE:
            category = CAT_READINESS_PROBE;
            m_threads_field_category.write_value(tw, thread_entry, category);
			break;
		}
        return;
    }
}

void my_plugin::on_new_process(const falcosecurity::table_entry& thread_entry,
                               const falcosecurity::table_reader& tr,
                               const falcosecurity::table_writer& tw) {
	std::shared_ptr<container_info> info = nullptr;
    auto container_id = compute_container_id_for_thread(thread_entry, tr, info);
    m_container_id_field.write_value(tw, thread_entry, container_id);

    if (info != nullptr) {
        // Since the matcher also returned a container_info,
        // it means we do not expect to receive any metadata from the go-worker,
        // since the engine has no listener SDK.
        // Just send the event now.
        nlohmann::json j(info);
        generate_async_event(j.dump().c_str(), true, ASYNC_HANDLER_DEFAULT);

        // Immediately cache the container metadata
        m_containers[info->m_id] = info;
    }

    // Write thread category field
    if (!container_id.empty()) {
        auto it = m_containers.find(container_id);
        if (it != m_containers.end()) {
            auto cinfo = it->second;
            write_thread_category(cinfo, thread_entry, tr, tw);
        } else {
            SPDLOG_DEBUG("failed to write thread category, no container found for {}", container_id);
        }
    }
}

bool my_plugin::parse_new_process_event(const falcosecurity::parse_event_input& in) {
    // get tid
    auto thread_id = in.get_event_reader().get_tid();

    auto& tr = in.get_table_reader();
    auto& tw = in.get_table_writer();

    // - For execve/execveat we exclude failed syscall events (ret<0)
    // - For clone/fork/vfork/clone3 we exclude failed syscall events (ret<0)
    // res is first param
    auto res_param = get_syscall_evt_param(in.get_event_reader().get_buf(), 0);
    int64_t ret = 0;
    memcpy(&ret, res_param.param_pointer, sizeof(ret));
    if(ret < 0) {
        return false;
    }

    // retrieve the thread entry associated with this thread id
    try {
    	auto thread_entry = m_threads_table.get_entry(tr, thread_id);
        on_new_process(thread_entry, tr, tw);
    	return true;
    } catch (falcosecurity::plugin_exception &e) {
      	SPDLOG_ERROR("cannot attach container_id to new process event for the thread id '{}': {}",
                     thread_id, e.what());
        return false;
    }
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
            return parse_container_json_event(in);
        case PPME_CONTAINER_JSON_2_E:
            // large payload
            return parse_container_json_2_event(in);
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