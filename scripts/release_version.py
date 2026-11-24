"""Read the public version and sequential build used in signed releases."""
import re
import sys


def release_info(version):
    match = re.fullmatch(r'(0\.0\.0|[1-9][0-9]*\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*))-preview\.([1-9][0-9]{0,8})', version)
    if not match:
        raise ValueError(f"Invalid release version: {version}")
    base, build = match.groups()
    if base == "0.0.0":
        return "0." + build, int(build)
    major, minor, patch = base.split(".")
    return (".".join([major, minor]) if patch == "0" else base), int(build)


if __name__ == "__main__":
    print(release_info(sys.argv[1])[0])
