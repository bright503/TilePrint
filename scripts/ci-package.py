"""Package one CI target; run from the repository root. Uses only Python stdlib."""

import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import zipfile


def run(*args):
    subprocess.run(args, check=True)


def copy_one(pattern, destination):
    matches = list(Path("build").glob(pattern))
    if len(matches) != 1:
        raise RuntimeError(f"Expected one {pattern}, found {matches}")
    shutil.copy2(matches[0], destination)


def main():
    target = sys.argv[1]
    if target not in {"windows-amd64", "darwin-arm64", "darwin-amd64", "linux-amd64"}:
        raise ValueError(f"Unsupported target: {target}")
    config_path = Path("mygo.json")
    config = json.loads(config_path.read_text(encoding="utf-8"))
    if os.environ.get("GITHUB_REF_TYPE") == "tag":
        tag = os.environ["GITHUB_REF_NAME"]
        if not re.fullmatch(r"v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?", tag):
            raise ValueError("Release tags must be vMAJOR.MINOR.PATCH[-prerelease]")
        config["version"] = tag[1:]
    version = config["version"]
    original = config_path.read_bytes()
    dist = Path("dist")
    dist.mkdir(exist_ok=True)
    try:
        # Only the runner's temporary checkout is changed; the source stays unchanged.
        config_path.write_text(json.dumps(config, indent=2) + "\n", encoding="utf-8")
        prefix = f"tileprint-{version}-{target}"
        if target == "windows-amd64":
            run("go", "run", "scripts/package-windows.go")
            copy_one("windows-online/windows-amd64/tileprint-setup-*.exe", dist / f"{prefix}-setup-online.exe")
            # Zip the executable and its resource files, excluding setup/uninstall files.
            stage = Path("build/windows-online/windows-amd64")
            with zipfile.ZipFile(dist / f"{prefix}-portable.zip", "w", zipfile.ZIP_DEFLATED) as archive:
                for path in sorted(stage.rglob("*")):
                    if path.is_file() and not path.name.startswith(("tileprint-setup-", "uninstall")):
                        archive.write(path, path.relative_to(stage))
            run("go", "run", "scripts/package-windows.go", "-offline")
            copy_one("windows-offline/windows-amd64/tileprint-setup-*.exe", dist / f"{prefix}-setup-offline.exe")
        else:
            run("go", "run", "github.com/egoist/mygo/cmd/mygo@v0.8.3", "build", "-platform", target.replace("-", "/"))
            if target.startswith("darwin-"):
                copy_one(f"{target}/*.dmg", dist / f"{prefix}.dmg")
                run("ditto", "-c", "-k", "--sequesterRsrc", "--keepParent",
                    f"build/{target}/TilePrint.app", str(dist / f"{prefix}.app.zip"))
            else:
                copy_one(f"{target}/*.tar.gz", dist / f"{prefix}.tar.gz")
                copy_one(f"{target}/*.deb", dist / f"{prefix}.deb")
    finally:
        config_path.write_bytes(original)


if __name__ == "__main__":
    main()
