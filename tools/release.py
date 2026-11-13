#!/usr/bin/env python3
"""Prepare a tag build, validate its package matrix, and write SHA256SUMS."""
import argparse
import hashlib
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[1]
STEM = "viva-agents-manager"
FORMATS = {
    "darwin": ("{stem}-{version}-darwin-{arch}.dmg", "{stem}-{version}-darwin-{arch}.zip"),
    "windows": ("{stem}-{version}-windows-{arch}-setup.exe", "{stem}-{version}-windows-{arch}.zip"),
    "linux": ("{stem}-{version}-linux-{arch}.tar.gz", "{stem}_{version}_{arch}.deb"),
}


def version_of(tag):
    if not re.fullmatch(r"v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)", tag):
        raise ValueError(f"expected vX.X.X, got {tag!r}")
    return tag[1:]


def expected(version, target=None, arch=None):
    return {
        pattern.format(stem=STEM, version=version, arch=architecture)
        for platform in ([target] if target else FORMATS)
        for architecture in ([arch] if arch else ("amd64", "arm64"))
        for pattern in FORMATS[platform]
    }


def verify(directory, names):
    actual = {path.name for path in directory.iterdir() if path.is_file()}
    if actual != names:
        raise ValueError(f"package matrix mismatch; missing={sorted(names - actual)}, unexpected={sorted(actual - names)}")
    for name in names:
        if (directory / name).stat().st_size == 0:
            raise ValueError(f"empty package: {name}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("prepare", "verify", "checksums"))
    parser.add_argument("tag")
    parser.add_argument("--dir", type=pathlib.Path)
    parser.add_argument("--target", choices=FORMATS)
    parser.add_argument("--arch", choices=("amd64", "arm64"))
    args = parser.parse_args()
    version = version_of(args.tag)
    if args.command == "prepare":
        manifest = ROOT / "viva.toml"
        content, count = re.subn(r'^version\s*=\s*"[^"\n]*"\s*$', f'version = "{version}"', manifest.read_text(), flags=re.MULTILINE)
        if count != 1:
            raise ValueError("viva.toml must contain exactly one version")
        manifest.write_text(content)
        (ROOT / "internal/version/version.txt").write_text(version + "\n")
        print(f"Preparing {args.tag}")
        return
    if args.dir is None:
        parser.error("--dir is required")
    if bool(args.target) != bool(args.arch):
        parser.error("--target and --arch must be supplied together")
    if args.command == "checksums" and args.target:
        parser.error("checksums requires the complete platform matrix")
    names = expected(version, args.target, args.arch)
    verify(args.dir, names)
    if args.command == "checksums":
        lines = []
        for name in sorted(names):
            with (args.dir / name).open("rb") as stream:
                digest = hashlib.file_digest(stream, "sha256").hexdigest()
            lines.append(f"{digest}  {name}\n")
        (args.dir / "SHA256SUMS").write_text("".join(lines))
    print(f"Verified {len(names)} packages for {args.tag}")


if __name__ == "__main__":
    main()
