# TODO

Remaining unsupported fields:
* CRI:
  - [ ] CniJson
* Containerd:
  - [ ] Ip
  - [ ] fix image related fields being empty when a docker container is spawned

- [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early
- [ ] Implement support for [`cri_extra_queries`](https://github.com/falcosecurity/libs/blob/bd0bb9baf273acc346dec881ec1d264911d74893/userspace/libsinsp/cri.hpp#L837)? It is enabled by default and moreover it does not seem needed with current code

- [x] reimplement `sinsp_container_manager::identify_category()` : https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L488
  - [x] finish implementing identify_category logic
- [x] support container engines "hotplug", ie: container engine that gets started as soon as its socket becomes available
- [x] expose plugin containers cache (`user` and `ip` for now) as sinsp state API table
- [x] Implement containerd matcher
- [x] Implement some unit tests taken from sinsp