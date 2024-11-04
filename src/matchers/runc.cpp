#include "runc.h"

namespace {
const size_t CONTAINER_ID_LENGTH = 64;
const size_t REPORTED_CONTAINER_ID_LENGTH = 12;
const char *CONTAINER_ID_VALID_CHARACTERS = "0123456789abcdefABCDEF";

static_assert(REPORTED_CONTAINER_ID_LENGTH <= CONTAINER_ID_LENGTH,
"Reported container ID length cannot be longer than actual length");
}  // namespace


namespace libsinsp {
namespace runc {

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

    if(end_pos - start_pos != CONTAINER_ID_LENGTH) {
        return false;
    }

    size_t invalid_ch_pos = cgroup.find_first_not_of(CONTAINER_ID_VALID_CHARACTERS, start_pos);
    if(invalid_ch_pos < CONTAINER_ID_LENGTH) {
        return false;
    }

    container_id = cgroup.substr(start_pos, REPORTED_CONTAINER_ID_LENGTH);
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