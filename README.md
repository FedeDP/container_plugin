# Container metadata enrichment Plugin

## Experimental

Consider this plugin as experimental until it reaches version `1.0.0`. By 'experimental' we mean that, although the plugin is functional and tested, it is currently in active development and may undergo changes in behavior as necessary, without prioritizing backward compatibility.

## Introduction

The `container` plugin enhances the Falco syscall source by providing additional information about container resources involved. You can find the comprehensive list of supported fields [here](#supported-fields).

### Functionality

The plugin itself reimplements all the container-related logic that was already present in libs under the form of a plugin, that can be attached to any source.  
Moreover, it aims to fix issues present in the current implementation, trying to be as quick as possible to gather container metadata information, to avoid losing 
a single event metadata.

## Capabilities

The `container` plugin implements 3 capabilities:

* `extraction` -> to extract `container.X` related fields
* `parsing` -> to parse `async` and `container` events (the latter for backward compatibility with existing scap files), and clone/fork events
* `async` -> to generate events with container infos

## Architecture

The `container` plugin is split into 2 modules:
* a C++ shared object that implements the 3 capabilities and holds the cache map
* a GO static library (linked inside the C++ shared object) that implements the worker logic to retrieve new containers' metadata

Once the GO worker finds a new container, it immediately generates an `async` event through a callback that is passed by the C++ side, to enrich its own internal state cache.

Every time a clone/fork event gets parsed (ie: a new thread is created in the system), we attach to its thread table entry
the information about the container_id, extracted through a regex by looking at the `cgroups` field, in a foreign key.

Once the extraction is requested for a threadinfo, the container_id is then used as key to access our plugin's internal container metadata cache, and the requested infos extracted.

### Plugin official name

`container`

### Supported Fields

<!-- README-PLUGIN-FIELDS -->
| NAME                          | TYPE      | ARG                  | DESCRIPTION                          |
|-------------------------------|-----------|----------------------|--------------------------------------|
| `container.id`                | `string`  | None                 | Container ID.                        |
| `container.full_id`           | `string`  | None                 | Container ID.                        |
| `container.name`              | `string`  | None                 | Container name.                      |
| `container.image`             | `string`  | None                 | Image name.                          |
| `container.image.id`          | `string`  | None                 | Image ID.                            |
| `container.type`              | `string`  | None                 | Type.                                |
| `container.privileged`        | `bool`    | None                 | Privileged.                          |
| `container.mounts`            | `string`  | None                 | Mounts.                              |
| `container.mount`             | `string`  | Idx or Key, Required | Mount.                               |
| `container.mount.source`      | `string`  | Idx or Key, Required | Mount Source.                        |
| `container.mount.dest`        | `string`  | Idx or Key, Required | Mount Destination.                   |
| `container.mount.mode`        | `string`  | Idx or Key, Required | Mount Mode.                          |
| `container.mount.rdwr`        | `string`  | Idx or Key, Required | Mount Read/Write.                    |
| `container.mount.propagation` | `string`  | Idx or Key, Required | Mount Propagation.                   |
| `container.image.repository`  | `string`  | None                 | Repository.                          |
| `container.image.tag`         | `string`  | None                 | Image Tag.                           |
| `container.image.digest`      | `string`  | None                 | Registry Digest.                     |
| `container.healthcheck`       | `string`  | None                 | Health Check.                        |
| `container.liveness_probe`    | `string`  | None                 | Liveness.                            |
| `container.readiness_probe`   | `string`  | None                 | Readiness.                           |
| `container.start_ts`          | `abstime` | None                 | Container start.                     |
| `container.duration`          | `reltime` | None                 | Container duration.                  |
| `container.ip`                | `string`  | None                 | Container IP.                        |
| `container.cni.json`          | `string`  | None                 | Container's / pod's CNI result json. |
| `container.host_pid`          | `bool`    | None                 | Host PID Namespace.                  |
| `container.host_network`      | `bool`    | None                 | Host Network Namespace.              |
| `container.host_ipc`          | `bool`    | None                 | Host IPC Namespace.                  |
<!-- /README-PLUGIN-FIELDS -->

## Requirements

* `containerd` >= 1.7 (https://kubernetes.io/docs/tasks/administer-cluster/switch-to-evented-pleg/, https://github.com/containerd/containerd/pull/7073)
* `cri-o` >= 1.26 (https://kubernetes.io/docs/tasks/administer-cluster/switch-to-evented-pleg/)
* `podman` >= v2.0.0 (https://github.com/containers/podman/commit/165aef7766953cd0c0589ffa1abc25022a905adb)

One can customize those sockets through init config; moreover, one can disable each engine individually.

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
* Containerd: `/run/containerd/containerd.sock`
* Cri: [`/run/crio/crio.sock`, `/run/k3s/containerd/containerd.sock`]

### Rules

This plugin doesn't provide any custom rule, you can use the default Falco ruleset and add the necessary `container` fields. A very simple example rule can be found [here](https://github.com/falcosecurity/plugins/blob/main/plugins/k8smeta/test/rules/example_rule.yaml).
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

You can also run `make exe` from withing the `go-worker` folder to build a `worker` executable,
to test the go-worker implementation.