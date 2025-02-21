#include <plugin.h>

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
                try {
                   on_new_process(e, tr, tw);
                   return true;
                } catch (falcosecurity::plugin_exception &e) {
                    SPDLOG_ERROR("cannot attach container_id to process: {}", e.what());
                    return false;
                }
            });
    return true;
}

bool my_plugin::capture_close(const falcosecurity::capture_listen_input& in) {
    return true;
}

FALCOSECURITY_PLUGIN_CAPTURE_LISTENING(my_plugin);