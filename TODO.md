# TODO

## Plugin

- [x] reimplement `sinsp_container_manager::identify_category()` : https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L488
  - [ ] finish implementing identify_category logic
  - [x] implement TYPE_IS_CONTAINER_HEALTHCHECK, TYPE_IS_CONTAINER_LIVENESS_PROBE, TYPE_IS_CONTAINER_READINESS_PROBE extractors and make `theadinfo::m_category` a foreign key

- [x] install re2 as bundled dep (leverage vcpkg)
- [x] use vcpkg for spdlog too
- [x] build go-worker in the cmake module

- [x] support container engines "hotplug", ie: container engine that gets started as soon as its socket becomes available

- [ ] expose plugin containers cache as sinsp state API table; needed by `sinsp_network_interfaces::is_ipv4addr_in_local_machine()` :/

- [ ] properly send json with all info from go-worker
    - [ ] fix remaining TODOs
    - [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early
    - [ ] send healthprobe related infos

## Falco

- [ ] double check %container.info: https://github.com/falcosecurity/falco/blob/master/userspace/engine/rule_loader_compiler.cpp#L38
    - [ ] set to empty the `s_default_extra_fmt` for `%container.info` in falco rule_loader_compiler
    - [x] the plugin will enforce `container.id and container.name` in output through the new plugin API
- [ ] if container plugin is not loaded but a rule requires `container.*` fields, register a stub plugin (like we do in tests)
  that just returns "n/a", "", "host": since default Falco rules use container fields, we need a way to ensure that they load fine even if container plugin is not loaded
- [ ] Rules `- required_engine_version: ` bump so that old rules stop working with new Falco, and we can fix all of them to avoid using `container.X` fields by default?
- [ ] Keep the container plugin inside the Falco bundle until Falco 1.0.0

## Libs

**Remove all container-related code from sinsp to be able to inject the plugin**

Ongoing upstream branch: `cleanup/drop_container_manager`

- [x] Dump related: -> https://github.com/falcosecurity/libs/pull/2152
    - [x] `sinsp_dumper::open()` calls `m_container_manager.dump_containers()` -> need new API to dump plugin state
    - [x] plugin must also be able to pre-parse all `PPME_CONTAINER(_JSON)_E` and `PPME_ASYNCEVENT_E` at capture open

- [ ] User group manager related: -> https://github.com/falcosecurity/libs/pull/2165
    - [ ] `sinsp_usergroup_manager::subscribe_container_mgr()` has a listener on container removed/added -> possible fix: parse `ASYNCEVENT("container_{added,removed}") instead
    - [ ] multiple parsers use `m_container_id` field to refresh tinfo user/loginuser/group information
    - [ ] `sinsp_threadinfo::set_{user/group/loginuser}()` use container_id
    - [ ] `sinsp_filter_check_user::extract_single()` needs container_id to return `user.name` field
    - [ ] fix for all of the above: add a small threadinfo method `get_container_id` that uses libsinsp state table API to retrieve the container id set by the plugin

- [ ] Others
    - [x] [`sinsp_observer::on_resolve_container`](https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/sinsp_observer.h#L54) -> used by agent: https://github.com/search?q=repo%3Adraios%2Fagent%20on_resolve_container&type=code -> KILL IT
    - [ ] [`sinsp_threadinfo::compute_program_hash()`](https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/threadinfo.cpp#L209) uses `container_id` -> add a small threadinfo method `get_container_id` that uses libsinsp state table API to retrieve it
    - [ ] [`sinsp_network_interfaces::is_ipv4addr_in_local_machine()`](https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/ifinfo.cpp#L217) uses container_manager. Used by sinsp_filtercheck_fd to determine if a socket is local -> Expose container cache as state table API from the plugin with only needed fields (in this case, container IP)
    - [x] [`sinsp_container_manager::identify_category`](https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L488) needs to be reimplemented by the plugin (lookup hashing plugin for the threads parent loop)
    - [x] drop `TYPE_IS_CONTAINER_HEALTHCHECK`, `TYPE_IS_CONTAINER_LIVENESS_PROBE`, `TYPE_IS_CONTAINER_READINESS_PROBE` extractors
    - [x] drop threadinfo::m_category

- [ ] Tests
    - [ ] all Libs container-related tests? Will need to run sinsp with the external plugin!
    - [ ] we will need to run sinsp-example with plugins -> extend it! (for testing purposes and for e2e test framework by Mauro)
