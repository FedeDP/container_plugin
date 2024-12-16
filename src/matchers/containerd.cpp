#include "containerd.h"
#include "runc.h"

using namespace libsinsp::runc;

constexpr const cgroup_layout CONTAINERD_CGROUP_LAYOUT[] = {{"/default/", ""}, {nullptr, nullptr}};

bool containerd::resolve(const std::string& cgroup, std::string& container_id) {
    return matches_runc_cgroup(cgroup, CONTAINERD_CGROUP_LAYOUT, container_id);
}