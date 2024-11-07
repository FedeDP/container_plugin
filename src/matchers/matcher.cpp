#include "matcher.h"
#include "docker.h"
#include "bpm.h"
#include "podman.h"
#include "cri.h"
#include "lxc.h"
#include "libvirt_lxc.h"
#include "static_container.h"

matcher_manager::matcher_manager(uint64_t container_engine_mask, const std::string& static_id, const std::string& static_name, const std::string& static_image) {
    if(container_engine_mask & (1 << CT_STATIC)) {
        // Configured with a static engine; add it and return.
        auto engine = std::make_shared<static_container>(static_id,
                                                                           static_name,
                                                                           static_image);
        m_matchers.push_back(engine);
        return;
    }

    if(container_engine_mask & (1 << CT_PODMAN)) {
        auto podman_engine = std::make_shared<podman>();
        m_matchers.push_back(podman_engine);
    }
    if(container_engine_mask & (1 << CT_DOCKER)) {
        auto docker_engine = std::make_shared<docker>();
        m_matchers.push_back(docker_engine);
    }
    if(container_engine_mask & ((1 << CT_CRI) | (1 << CT_CRIO) | (1 << CT_CONTAINERD))) {
        auto cri_engine = std::make_shared<cri>();
        m_matchers.push_back(cri_engine);
    }
    if(container_engine_mask & (1 << CT_LXC)) {
        auto lxc_engine = std::make_shared<lxc>();
        m_matchers.push_back(lxc_engine);
    }
    if(container_engine_mask & (1 << CT_LIBVIRT_LXC)) {
        auto libvirt_lxc_engine = std::make_shared<libvirt_lxc>();
        m_matchers.push_back(libvirt_lxc_engine);
    }
    if(container_engine_mask & (1 << CT_BPM)) {
        auto bpm_engine = std::make_shared<bpm>();
        m_matchers.push_back(bpm_engine);
    }
}

bool matcher_manager::match_cgroup(const std::string& cgroup, std::string& container_id,
                                   std::shared_ptr<container_info>& ctr) {
    for (const auto &matcher : m_matchers) {
        if (matcher->resolve(cgroup, container_id)) {
            ctr = matcher->to_container(container_id);
            return true;
        }
    }
    return false;
}