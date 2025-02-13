# TODO

Remaining unsupported fields:
* CRI:
  - [ ] CniJson
* Containerd:
  - [ ] Ip
  - [ ] fix image related fields being empty when a docker container is spawned

- [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early
- [ ] Implement support for [`cri_extra_queries`](https://github.com/falcosecurity/libs/blob/bd0bb9baf273acc346dec881ec1d264911d74893/userspace/libsinsp/cri.hpp#L837)? It is enabled by default and moreover it does not seem needed with current code
