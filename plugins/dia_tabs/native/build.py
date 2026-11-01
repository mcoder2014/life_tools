#!/usr/bin/env python3
"""Create clean extension, Alfred and portable source archives."""

import argparse
from pathlib import Path
import re
import zipfile

ROOT = Path(__file__).resolve().parents[1]


def add_tree(archive, source, prefix=""):
    for path in sorted(source.rglob("*")):
        relative = path.relative_to(source)
        if (not path.is_file() or path.is_symlink() or any(part.startswith(".") for part in relative.parts)
                or "__pycache__" in relative.parts or path.suffix in (".pyc", ".sock")
                or path.name in ("prefs.plist", "install-state.json", "python.path")):
            continue
        archive.write(path, str(Path(prefix) / relative))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("output", nargs="?", type=Path, default=ROOT / "dist")
    parser.add_argument("version", nargs="?", default="dev")
    args = parser.parse_args()
    if not re.fullmatch(r"[A-Za-z0-9._-]+", args.version):
        parser.error("version can contain only letters, digits, dots, underscores and hyphens")
    args.output.mkdir(parents=True, exist_ok=True)
    stem = "life_tools_dia_tabs_"
    artifacts = [("extension_" + args.version + ".zip", ROOT / "extension"),
                 (args.version + ".alfredworkflow", ROOT / "workflow")]
    for name, source in artifacts:
        path = args.output / (stem + name)
        with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
            add_tree(archive, source)
        print(path.resolve())
    path = args.output / (stem + "source_" + args.version + ".zip")
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
        prefix = stem + args.version
        for component in ("extension", "native", "workflow"):
            add_tree(archive, ROOT / component, prefix + "/" + component)
        for name in ("build.sh", "install.sh"):
            archive.write(ROOT / name, prefix + "/" + name)
        readme = ROOT.parents[1] / "docs/plugins/dia_tabs.md"
        if not readme.exists():
            readme = ROOT / "README.md"
        if readme.exists():
            archive.write(readme, prefix + "/README.md")
    print(path.resolve())


if __name__ == "__main__":
    main()
