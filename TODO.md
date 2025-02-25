# TODO

Remaining unsupported fields:
* Containerd:
  - [ ] Ip

- [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early
- [ ] ?? merge existing containers instead of always replacing (ie: if 2 engines add the same container)
- [ ] non-listeners engines are never removed from plugin cache