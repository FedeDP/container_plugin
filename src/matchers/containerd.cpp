#include "containerd.h"
#include "runc.h"

using namespace libsinsp::runc;

// Containers created via ctr
// use the "default" namespace (instead of the cri "k8s.io" namespace)
// which will result in the `/default` cgroup path.
// https://github.com/containerd/containerd/blob/3b15606e196e450cf817fa9f835ab5324b35a28b/pkg/namespaces/context.go#L32
constexpr const cgroup_layout CONTAINERD_CGROUP_LAYOUT[] = {{"/default/", ""}, {nullptr, nullptr}};

bool containerd::resolve(const std::string& cgroup, std::string& container_id) {
    return matches_runc_cgroup(cgroup, CONTAINERD_CGROUP_LAYOUT, container_id);
}