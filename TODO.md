# TODO

## Plugin
- [x] attach also execve/execveat etc etc (basically check wherever `resolve_container` is used in current libs code)
- [x] implement initial proc parsing logic to attach container_id foreign key to existing threads leveraging capture listener API
- [x] implement sinsp_filtercheck_k8s.cpp filterchecks: https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/sinsp_filtercheck_k8s.cpp#L364
    - [x] implement deprecated fields too? https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/sinsp_filtercheck_k8s.h#L36
- [x] rewrite container_info.cpp logic to parse the new json sent by coworker
    - [x] Drop jsoncpp dep and use nlohmann since it is already in use by the plugin-sdk-cpp
    - [x] just implement `to_json` and `from_json` on the class (like `PluginConfig`)
    - [ ] somehow get rid of re2 usage in `mount_by_source` and `mount_by_dest` to drop the dep
    - [x] make sure that the new container json is exactly the same as the old one
    - [x] implement healthprobes logic
    - [x] reimplement `sinsp_container_manager::identify_category()` : https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L488
    - [ ] finish implementing identify_category logic
    - [x] implement TYPE_IS_CONTAINER_HEALTHCHECK, TYPE_IS_CONTAINER_LIVENESS_PROBE, TYPE_IS_CONTAINER_READINESS_PROBE extractors and make `theadinfo::m_category` a foreign key
    - [ ] expose plugin containers cache as sinsp state API table
    - [ ] `threadinfo::m_parent_loop_detected`?? 

- [x] implement new init config key: `label_max_len: 100 # (optional, default: 100; container labels larger than this won't be reported)`

- [x] implement container.labels[] support

- [x] improve logging

- [x] properly send json with all info from go-worker
    - [x] docker
    - [x] podman
    - [x] containerd
    - [x] cri
    - [x] port CreatedAt to int64
    - [ ] fix remaining TODOs
    - [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early :/
    - [ ] send healthprobe related infos
    - [x] add tests

- [x] implement correct logic to extract container_id for each container_engine like we do in current sinsp impl
    - [x] implement container runtimes that only use the container id/type, like rkt,bpm,libvirt,lxc, in the C++ side since we don't have a listener API
    - [x] add support for hidden (ie: non-documented) "static" container matcher in init config

- [x] parameterize `get_fields` fields to allow to change PLUGIN_NAME for testing purposes

- [x] fixup CRI `GetContainerEvents()` (not sending any event) ??

## Falco

- [ ] double check %container.info: https://github.com/falcosecurity/falco/blob/master/userspace/engine/rule_loader_compiler.cpp#L38
    - [ ] set to empty the `s_default_extra_fmt` for `%container.info` in falco rule_loader_compiler
    - [x] the plugin will enforce `container.id and container.name` in output through the new plugin API
- [ ] if container plugin is not loaded but a rule requires `container.*` fields, register a stub plugin (like we do in tests)
  that just returns "n/a", "", "host": since default Falco rules use container fields, we need a way to ensure that they load fine even if container plugin is not loaded

## Libs

**Remove all container-related code from sinsp to be able to inject the plugin**

- [x] Dump related
    - [x] `sinsp_dumper::open()` calls `m_container_manager.dump_containers()` -> need new API to dump plugin state (see https://github.com/falcosecurity/libs/pull/2152)
    - [x] plugin must also be able to pre-parse all `PPME_CONTAINER(_JSON)_E` and `PPME_ASYNCEVENT_E` at capture open (see https://github.com/falcosecurity/libs/pull/2152)

- [ ] User group manager related: -> https://github.com/falcosecurity/libs/pull/2165
    - [ ] `sinsp_usergroup_manager::subscribe_container_mgr()` has a listener on container removed/added -> possible fix: parse `ASYNCEVENT("container_{added,removed}") instead
    - [ ] multiple parsers use `m_container_id` field to refresh tinfo user/loginuser/group information
    - [ ] `sinsp_threadinfo::set_{user/group/loginuser}()` use container_id
    - [ ] `sinsp_filter_check_user::extract_single()` needs container_id to return `user.name` field
    - [ ] fix for all of the above: keep container_id in threadinfo but fill it from the plugin, or add a small threadinfo method `get_container_id` that uses libsinsp state table API to retrieve it.

- [ ] Others
    - [ ] `sinsp_threadinfo::compute_program_hash()` uses container_id -> expose `m_program_hash` and `m_program_hash_scripts` from libs and compute the hash in the plugin? Or better still, just add the fields from the plugin like we do for container_id
    - [ ] `sinsp_network_interfaces::is_ipv4addr_in_local_machine()` uses container_manager. Used by sinsp_filtercheck_fd to determine if local socket... ???

- [ ] Tests
    - [ ] all Libs container-related tests? Will need to run sinsp with the external plugin!
    - [ ] we will need to run sinsp-example with plugins -> extend it! (for testing purposes and for e2e test framework by Mauro)
