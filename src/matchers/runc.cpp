#include "runc.h"

namespace {
const size_t CONTAINER_ID_LENGTH = 64;
const size_t REPORTED_CONTAINER_ID_LENGTH = 12;

static_assert(REPORTED_CONTAINER_ID_LENGTH <= CONTAINER_ID_LENGTH,
"Reported container ID length cannot be longer than actual length");
}  // namespace


namespace libsinsp {
namespace runc {

inline static bool endswith(const std::string &s, const std::string &suffix) {
	return s.rfind(suffix) == (s.size() - suffix.size());
}

inline static bool is_host(const std::string &cgroup) {
	// A good approximation to minize false-positives is to exclude systemd suffixes.
	if(endswith(cgroup, ".slice") || endswith(cgroup, ".service")) {
		return true;
	} else if(endswith(cgroup, ".scope")) {
		if(cgroup.find("crio-") != std::string::npos ||
		   cgroup.find("docker-") != std::string::npos) {
			return false;
		}
		return true;
	}

	return false;
}

// check if cgroup ends with <prefix><container_id><suffix>
// If true, set <container_id> to a truncated version of the id and return true.
// Otherwise return false and leave container_id unchanged
bool match_one_container_id(const std::string &cgroup,
                            const std::string &prefix,
                            const std::string &suffix,
                            std::string &container_id) {
    size_t start_pos = cgroup.rfind(prefix);
    if(start_pos == std::string::npos) {
        return false;
    }
    start_pos += prefix.size();

    size_t end_pos = cgroup.rfind(suffix);
    if(end_pos == std::string::npos) {
        return false;
    }

    // In some container runtimes the container id is not
    // necessarly CONTAINER_ID_LENGTH long and can be arbitrarly defined.
    // To keep it simple we only discard the container id > of CONTAINER_ID_LENGTH.
    if(end_pos - start_pos > CONTAINER_ID_LENGTH || end_pos - start_pos == 0) {
        return false;
    }

    if(is_host(cgroup)) {
        return false;
    }

    size_t reported_len = end_pos - start_pos >= REPORTED_CONTAINER_ID_LENGTH
                                  ? REPORTED_CONTAINER_ID_LENGTH
                                  : end_pos;
    container_id = cgroup.substr(start_pos, reported_len);
    return true;
}

bool matches_runc_cgroup(const std::string &cgroup,
                        const libsinsp::runc::cgroup_layout *layout,
                        std::string &container_id) {
    for(size_t i = 0; layout[i].prefix && layout[i].suffix; ++i) {
        if(match_one_container_id(cgroup, layout[i].prefix, layout[i].suffix, container_id)) {
            return true;
        }
    }
    return false;
}
}  // namespace runc
}  // namespace libsinsp