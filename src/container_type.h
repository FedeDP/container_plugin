#pragma once

enum container_type {
    CT_DOCKER = 0, // TODO implement matcher
    CT_LXC = 1,
    CT_LIBVIRT_LXC = 2,
    CT_MESOS = 3, // deprecated
    CT_RKT = 4, // deprecated
    CT_CUSTOM = 5,
    CT_CRI = 6, // TODO implement matcher
    CT_CONTAINERD = 7, // TODO implement matcher
    CT_CRIO = 8, // TODO implement matcher
    CT_BPM = 9,
    CT_STATIC = 10, // TODO implement matcher
    CT_PODMAN = 11, // TODO implement matcher

    // Default value, may be changed if necessary
    CT_UNKNOWN = 0xffff
};

static std::string inline to_string(enum container_type ct) {
    switch(ct) {
    case CT_DOCKER:
        return "docker";
        break;
    case CT_LXC:
        return "lxc";
        break;
    case CT_LIBVIRT_LXC:
        return "libvirt-lxc";
        break;
    case CT_MESOS:
        return "mesos";
        break;
    case CT_CRI:
        return "cri";
        break;
    case CT_CONTAINERD:
        return "containerd";
        break;
    case CT_CRIO:
        return "cri-o";
        break;
    case CT_RKT:
        return "rkt";
        break;
    case CT_BPM:
        return "bpm";
        break;
    case CT_PODMAN:
        return "podman";
        break;
    default:
        return "";
    }
}