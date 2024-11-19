# Container metadata enrichment Plugin

## TODO

### Plugin
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
  - [ ] reimplement `sinsp_container_manager::identify_category()` : https://github.com/falcosecurity/libs/blob/master/userspace/libsinsp/container.cpp#L488

- [x] implement new init config key: `label_max_len: 100 # (optional, default: 100; container labels larger than this won't be reported)`

- [x] implement container.labels[] support

- [x] properly send json with all info from go-worker
  - [x] docker
  - [x] podman
  - [x] containerd
  - [x] cri
  - [x] port CreatedAt to int64
  - [ ] fix remaining TODOs
  - [ ] fix: docker is not able to retrieve IP because onContainerCreate is called too early :/
  - [ ] send healthprobe related infos

- [x] implement correct logic to extract container_id for each container_engine like we do in current sinsp impl
  - [x] implement container runtimes that only use the container id/type, like rkt,bpm,libvirt,lxc, in the C++ side since we don't have a listener API
  - [x] add support for hidden (ie: non-documented) "static" container matcher in init config

- [x] parameterize `get_fields` fields to allow to change PLUGIN_NAME for testing purposes

- [ ] fixup CRI `GetContainerEvents()` (not sending any event) ??

### Falco

- [ ] double check %container.info: https://github.com/falcosecurity/falco/blob/master/userspace/engine/rule_loader_compiler.cpp#L38
    - [ ] set to empty the `s_default_extra_fmt` for `%container.info` in falco rule_loader_compiler
    - [x] the plugin will enforce `container.id and container.name` in output through the new plugin API
- [ ] if container plugin is not loaded but a rule requires `container.*` fields, register a stub plugin (like we do in tests) 
that just returns "n/a", "", "host": since default Falco rules use container fields, we need a way to ensure that they load fine even if container plugin is not loaded

### Libs

**Remove all container-related code from sinsp to be able to inject the plugin**

- [x] Dump related
  - [x] `sinsp_dumper::open()` calls `m_container_manager.dump_containers()` -> need new API to dump plugin state (see https://github.com/falcosecurity/libs/pull/2152)
  - [x] plugin must also be able to pre-parse all `PPME_CONTAINER(_JSON)_E` and `PPME_ASYNCEVENT_E` at capture open (see https://github.com/falcosecurity/libs/pull/2152)

- [ ] User group manager related:
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

## Experimental

Consider this plugin as experimental until it reaches version `1.0.0`. By 'experimental' we mean that, although the plugin is functional and tested, it is currently in active development and may undergo changes in behavior as necessary, without prioritizing backward compatibility.

## Introduction

The `container` plugin enhances the Falco syscall source by providing additional information about container resources involved. You can find the comprehensive list of supported fields [here](#supported-fields).

### Functionality

The plugin itself reimplements all the container-related logic that was already present in libs under the form of a plugin, that can be attached to any source.  
Moreover, it aims to fix issues present in the current implementation, trying to be as quick as possible to gather container metadata information, to avoid losing 
a single event metadata.

## Capabilities

The `container` plugin implements the following capabilities:

* `capture listening` -> to attach `container_id` foreign key to all pre-existing threadinfos, once they have been scraped from procfs by sinsp 
* `extraction` -> to extract `container.X` fields
* `parsing` -> to parse `async` and `container` events (the latter for backward compatibility with existing scap files), and clone/fork/execve events to attach `container_id` foreign key to any threads
* `async` -> to generate events with container information and `dump` current plugin cache state when requested

It requires **3.10.0** plugin API version.

## Architecture

The `container` plugin is split into 2 modules:
* a [C++ shared object](src) that implements the 3 capabilities and holds the cache map `<container_id,container_info>`
* a [GO static library](go-worker) (linked inside the C++ shared object) that implements the worker logic to retrieve new containers' metadata leveraging existing SDKs

As soon as the plugin starts, the go-worker gets started as part of the `async` capability, passing to it plugin init config and a C++ callback to generate async events. 
Whenever the GO worker finds a new container, it immediately generates an `async` event through the aforementioned callback.
The `async` event is then received by the C++ side as part of the `parsing` capability, and it enriches its own internal state cache.
Every time a clone/fork/execve event gets parsed, we attach to its thread table entry the information about the container_id, extracted through a regex by looking at the `cgroups` field, in a foreign key.
Once the extraction is requested for a thread, the container_id is then used as key to access our plugin's internal container metadata cache, and the requested infos extracted.

Note, however, that for some container engines, namely `{bpm,lxc,libvirt_lcx}`, we only support fetching generic info, ie: the container ID and the container type.  
Given that there is no "listener" SDK to attach to, for these engines the `async` event is generated directly by the C++ code, as soon as the container ID is retrieved.

### Plugin official name

`container`

### Supported Fields

<!-- README-PLUGIN-FIELDS -->
| NAME                          | TYPE      | ARG                  | DESCRIPTION                                |
|-------------------------------|-----------|----------------------|--------------------------------------------|
| `container.id`                | `string`  | None                 | Container ID (first 12B).                  |
| `container.full_id`           | `string`  | None                 | Container ID.                              |
| `container.name`              | `string`  | None                 | Container name.                            |
| `container.image`             | `string`  | None                 | Image name.                                |
| `container.image.id`          | `string`  | None                 | Image ID.                                  |
| `container.type`              | `string`  | None                 | Type.                                      |
| `container.privileged`        | `bool`    | None                 | Privileged.                                |
| `container.mounts`            | `string`  | None                 | Mounts.                                    |
| `container.mount`             | `string`  | Idx or Key, Required | Mount.                                     |
| `container.mount.source`      | `string`  | Idx or Key, Required | Mount Source.                              |
| `container.mount.dest`        | `string`  | Idx or Key, Required | Mount Destination.                         |
| `container.mount.mode`        | `string`  | Idx or Key, Required | Mount Mode.                                |
| `container.mount.rdwr`        | `string`  | Idx or Key, Required | Mount Read/Write.                          |
| `container.mount.propagation` | `string`  | Idx or Key, Required | Mount Propagation.                         |
| `container.image.repository`  | `string`  | None                 | Repository.                                |
| `container.image.tag`         | `string`  | None                 | Image Tag.                                 |
| `container.image.digest`      | `string`  | None                 | Registry Digest.                           |
| `container.healthcheck`       | `string`  | None                 | Health Check.                              |
| `container.liveness_probe`    | `string`  | None                 | Liveness.                                  |
| `container.readiness_probe`   | `string`  | None                 | Readiness.                                 |
| `container.start_ts`          | `abstime` | None                 | Container start.                           |
| `container.duration`          | `reltime` | None                 | Container duration.                        |
| `container.ip`                | `string`  | None                 | Container IP.                              |
| `container.cni.json`          | `string`  | None                 | Container's / pod's CNI result json.       |
| `container.host_pid`          | `bool`    | None                 | Host PID Namespace.                        |
| `container.host_network`      | `bool`    | None                 | Host Network Namespace.                    |
| `container.host_ipc`          | `bool`    | None                 | Host IPC Namespace.                        |
| `container.label`             | `string`  | Key, Required        | Container Label                            |
| `container.labels`            | `string`  | None                 | Container Labels                           |
| `k8s.pod.name`                | `string`  | None                 | Pod Name                                   |
| `k8s.ns.name`                 | `string`  | None                 | Namespace Name                             |
| `k8s.pod.id`                  | `string`  | None                 | Legacy Pod ID                              |
| `k8s.pod.uid`                 | `string`  | None                 | Pod UID                                    |
| `k8s.pod.sandbox_id`          | `string`  | None                 | Pod / Sandbox ID (first 12 chars)          |
| `k8s.pod.full_sandbox_id`     | `string`  | None                 | Pod / Sandbox ID                           |
| `k8s.pod.label`               | `string`  | Key, Required        | Pod Label                                  |
| `k8s.pod.labels`              | `string`  | None                 | Pod Labels                                 |
| `k8s.pod.ip`                  | `string`  | None                 | Pod Ip                                     |
| `k8s.pod.cni.json`            | `string`  | None                 | Pod CNI result json                        |
| `k8s.rc.name`                 | `string`  | None                 | [Deprecated] Replication Controller Name   |
| `k8s.rc.id`                   | `string`  | None                 | [Deprecated] Replication Controller ID     |
| `k8s.rc.label`                | `string`  | Key, Required        | [Deprecated] Replication Controller Label  |
| `k8s.rc.labels`               | `string`  | None                 | [Deprecated] Replication Controller Labels |
| `k8s.svc.name`                | `string`  | None                 | [Deprecated] Service Name                  |
| `k8s.svc.id`                  | `string`  | None                 | [Deprecated] Service ID                    |
| `k8s.svc.label`               | `string`  | Key, Required        | [Deprecated] Service Label                 |
| `k8s.svc.labels`              | `string`  | None                 | [Deprecated] Service Labels                |
| `k8s.ns.id`                   | `string`  | None                 | [Deprecated] Namespace ID                  |
| `k8s.ns.label`                | `string`  | Key, Required        | [Deprecated] Namespace Label               |
| `k8s.ns.labels`               | `string`  | None                 | [Deprecated] Namespace Labels              |
| `k8s.rs.name`                 | `string`  | None                 | [Deprecated] Replica Set Name              |
| `k8s.rs.id`                   | `string`  | None                 | [Deprecated] Replica Set ID                |
| `k8s.rs.label`                | `string`  | Key, Required        | [Deprecated] Replica Set Label             |
| `k8s.rs.labels`               | `string`  | None                 | [Deprecated] Replica Set Labels            |
| `k8s.deployment.name`         | `string`  | None                 | [Deprecated] Deployment Name               |
| `k8s.deployment.id`           | `string`  | None                 | [Deprecated] Deployment ID                 |
| `k8s.deployment.label`        | `string`  | Key, Required        | [Deprecated] Deployment Label              |
| `k8s.deployment.labels`       | `string`  | None                 | [Deprecated] Deployment Labels             |
 
<!-- /README-PLUGIN-FIELDS -->

## Requirements

* `containerd` >= 1.7 (https://kubernetes.io/docs/tasks/administer-cluster/switch-to-evented-pleg/, https://github.com/containerd/containerd/pull/7073)
* `cri-o` >= 1.26 (https://kubernetes.io/docs/tasks/administer-cluster/switch-to-evented-pleg/)
* `podman` >= v2.0.0 (https://github.com/containers/podman/commit/165aef7766953cd0c0589ffa1abc25022a905adb)

## Usage

### Configuration

Here's an example of configuration of `falco.yaml`:

> NOTE: Please note that you can provide values to the config as environment variables. So, for example, you can take advantage of the Kubernetes downward API to provide the node name as an env variable `nodename: ${MY_NODE}`.

```yaml
plugins:
  - name: container
    # path to the plugin .so file
    library_path: libcontainer.so
    init_config:
      # verbosity level for the plugin logger
      verbosity: warning # (optional, default: info)
      label_max_len: 100 # (optional, default: 100; container labels larger than this won't be reported)
      engines:
        docker:
          enabled: true
          sockets: ['/var/run/docker.sock']
        podman:
          enabled: true
          sockets: ['/run/podman/podman.sock', '/run/user/1000/podman/podman.sock']
        containerd:
          enabled: true
          sockets: ['/run/containerd/containerd.sock']
        cri:
          enabled: true
          sockets: ['/run/crio/crio.sock']
        lxc:
          enabled: false
        libvirt_lxc:
          enabled: false
        bpm:
          enabled: false  

load_plugins: [container]
```

**Default Sockets**:

* Docker: `/var/run/docker.sock`
* Podman: `/run/podman/podman.sock` for root, + `/run/user/$uid/podman/podman.sock` for each user in the system
* Containerd: [`/run/containerd/containerd.sock`, `/run/k3s/containerd/containerd.sock`]
* Cri: `/run/crio/crio.sock`

### Rules

This plugin doesn't provide any custom rule, you can use the default Falco ruleset and add the necessary `container` fields.
Note: leveraging latest plugin SDK features, the plugin itself will expose certain fields as suggested output fields:
* `container.id`
* `container.name`

### Running

This plugin requires Falco with version >= **0.40.0**.
Modify the `falco.yaml` with the [configuration above](#configuration) and you are ready to go!

```shell
falco -c falco.yaml -r falco_rules.yaml
```

## Local development

### Build and test

Build the plugin on a fresh `Ubuntu 22.04` machine:

```bash
sudo apt update -y
sudo apt install -y cmake build-essential autoconf libtool pkg-config
git clone https://github.com/falcosecurity/plugins.git
cd plugins/container
make libcontainer.so
```

You can also run `make exe` from withing the `go-worker` folder to build a `worker` executable to test the go-worker implementation.