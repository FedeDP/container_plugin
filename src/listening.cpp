#include "plugin.h"

//////////////////////////
// Listening capability
//////////////////////////

bool my_plugin::capture_open(const falcosecurity::capture_listen_input& in) {
    SPDLOG_DEBUG("enriching initial thread table entries");
    auto& tr = in.get_table_reader();
    auto& tw = in.get_table_writer();
    m_threads_table.iterate_entries(
            tr,
            [this, tr, tw](const falcosecurity::table_entry& e)
            {
                std::shared_ptr<container_info> info = nullptr;
                auto container_id = compute_container_id_for_thread(e, tr, info);
                m_container_id_field.write_value(tw, e,
                                                 (const char*)container_id.c_str());
                return true;
            });
    return true;
}

bool my_plugin::capture_close(const falcosecurity::capture_listen_input& in) {
    return true;
}

FALCOSECURITY_PLUGIN_CAPTURE_LISTENING(my_plugin);