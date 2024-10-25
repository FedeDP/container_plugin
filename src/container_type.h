#pragma once

enum sinsp_container_type {
    CT_DOCKER = 0,
    CT_LXC = 1,
    CT_LIBVIRT_LXC = 2,
    CT_MESOS = 3,
    CT_RKT = 4,
    CT_CUSTOM = 5,
    CT_CRI = 6,
    CT_CONTAINERD = 7,
    CT_CRIO = 8,
    CT_BPM = 9,
    CT_STATIC = 10,
    CT_PODMAN = 11,

    // Default value, may be changed if necessary
    CT_UNKNOWN = 0xffff
};