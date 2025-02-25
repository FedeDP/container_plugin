#include "containerd.h"
#include "runc.h"
#include <iostream>
#include <sstream> // for ostringstream
#include <re2/re2.h>

using namespace libsinsp::runc;

static re2::RE2 pattern("/([A-Za-z0-9]+(?:[._-](?:[A-Za-z0-9]+))*)/");

bool containerd::resolve(const std::string& cgroup, std::string& container_id)
{
    std::string containerd_namespace;
    // Containers created via ctr
    // use a cgroup path like: `0::/namespace/container_id`
    // Since we cannot know the namespace in advance, we try to
    // extract it from the cgroup path by following provided regex,
    // and use that to eventually extract the container id.
    if(re2::RE2::PartialMatch(cgroup, pattern, &containerd_namespace))
    {
        std::ostringstream out;
        out << "/" << containerd_namespace << "/";
        auto layout_str = out.str();
        cgroup_layout CONTAINERD_CGROUP_LAYOUT[] = {{layout_str.c_str(), ""},
                                                    {nullptr, nullptr}};
        return matches_runc_cgroup(cgroup, CONTAINERD_CGROUP_LAYOUT,
                                   container_id, true);
    }
    return false;
}