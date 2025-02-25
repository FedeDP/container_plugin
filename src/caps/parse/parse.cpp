#include <plugin.h>

//////////////////////////
// Parse capability
//////////////////////////

struct sinsp_param
{
    uint32_t param_len;
    uint8_t* param_pointer;
};

// Obtain a param from a sinsp event
template<const bool LargePayload = false,
         typename T = std::conditional_t<LargePayload, uint32_t*, uint16_t*>>
static inline sinsp_param get_syscall_evt_param(void* evt, uint32_t num_param)
{
    uint32_t dataoffset = 0;
    // pointer to the lengths array inside the event.
    auto len = (T)((uint8_t*)evt +
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
std::vector<std::string> my_plugin::get_parse_event_sources()
{
    return PARSE_EVENT_SOURCES;
}

std::vector<falcosecurity::event_type> my_plugin::get_parse_event_types()
{
    return PARSE_EVENT_CODES;
}

bool my_plugin::parse_async_event(const falcosecurity::parse_event_input& in)
{
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
    auto json_event = nlohmann::json::parse(json_charbuf_pointer);
    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    if(added)
    {
        SPDLOG_TRACE("Adding container: {}", cinfo->m_id);
        m_containers[cinfo->m_id] = cinfo;
        m_last_container = cinfo;
    }
    else
    {
        SPDLOG_TRACE("Removing container: {}", cinfo->m_id);
        m_containers.erase(cinfo->m_id);
    }

    // Update n_containers metric
    m_metrics.at(0).set_value((uint64_t)m_containers.size() - 1);

    // Update n_missing metric
    auto val = m_metrics.at(1).value.u64;
    if(!cinfo->m_is_pod_sandbox && cinfo->m_image.empty())
    {
        m_metrics.at(1).set_value(val + 1);
    }
    return true;
}

bool my_plugin::parse_container_event(
        const falcosecurity::parse_event_input& in)
{
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
        const falcosecurity::parse_event_input& in)
{
    auto& evt = in.get_event_reader();
    auto json_param = get_syscall_evt_param(evt.get_buf(), 0);

    std::string json_str = (char*)json_param.param_pointer;
    auto json_event = nlohmann::json::parse(json_str);

    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    SPDLOG_TRACE("Adding container from old container_json event: {}",
                 cinfo->m_id);
    m_containers[cinfo->m_id] = cinfo;
    m_last_container = cinfo;
    return true;
}

bool my_plugin::parse_container_json_2_event(
        const falcosecurity::parse_event_input& in)
{
    auto& evt = in.get_event_reader();
    auto json_param = get_syscall_evt_param<true>(evt.get_buf(), 0);

    std::string json_str = (char*)json_param.param_pointer;
    auto json_event = nlohmann::json::parse(json_str);

    auto cinfo = json_event.get<std::shared_ptr<container_info>>();
    SPDLOG_TRACE("Adding container from old container_json_2 event: {}",
                 cinfo->m_id);
    m_containers[cinfo->m_id] = cinfo;
    m_last_container = cinfo;
    return true;
}

bool my_plugin::parse_new_process_event(
        const falcosecurity::parse_event_input& in)
{
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
    if(ret < 0)
    {
        return false;
    }

    // retrieve the thread entry associated with this thread id
    try
    {
        auto thread_entry = m_threads_table.get_entry(tr, thread_id);
        on_new_process(thread_entry, tr, tw);
        return true;
    }
    catch(falcosecurity::plugin_exception& e)
    {
        SPDLOG_ERROR("cannot attach container_id to new process event for the "
                     "thread id '{}': {}",
                     thread_id, e.what());
        return false;
    }
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