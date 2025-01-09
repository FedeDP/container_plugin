# TODO

Remaining unsupported fields:
* CRI:
  - [ ] CniJson
* Containerd:
  - [ ] Ip
  - [ ] fix image related fields being empty when a docker container is spawned
- [ ] all healthprobe related infos

- [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early
- [ ] Implement support for [`cri_extra_queries`](https://github.com/falcosecurity/libs/blob/bd0bb9baf273acc346dec881ec1d264911d74893/userspace/libsinsp/cri.hpp#L837)? It is enabled by default and moreover it does not seem needed with current code

- [x] reimplement `sinsp_container_manager::identify_category()` : https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L488
  - [x] finish implementing identify_category logic

- [x] support container engines "hotplug", ie: container engine that gets started as soon as its socket becomes available

- [ ] expose plugin containers cache as sinsp state API table; needed by `sinsp_network_interfaces::is_ipv4addr_in_local_machine()` :/

- [ ] what to do with threads with empty container_id (ie: neither host nor id)? 
  - assume they are on host and return host info?
  - don't assume anything and just skip them?
  - it can happen 2 ways:
    * there is an interval of time after we scanned proc and before we start the sinsp capture, where new threads created get lost
    * if clone/exexve syscalls are lost the plugin won't receive them and thus container_id won't be written -> this already happens in sinsp
    * the latter can be fixed by letting `extract` write the foreign key in the threadtable, so that we store the thread container_id during extraction


- [x] Implement containerd matcher
- [x] Implement some unit tests taken from sinsp