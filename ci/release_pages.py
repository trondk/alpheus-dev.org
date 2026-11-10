#!/usr/bin/env python3
"""Make the pages under /releases/ from the releases' own files.

Usage: ci/release_pages.py <dir>

<dir> has a folder per release, vX.Y.Z/, with the release's
RELEASE-NOTES.md, the version's section of CHANGELOG.md that
hack/release.sh puts among the release's files (GO-65), and optionally its
SHA256SUMS, which lists the files. Writes content/releases/_index.md, the
list newest first, and content/releases/vX.Y.Z.md for each release, with
its notes, its files, and how long it gets security fixes (GO-134).
"""

import pathlib
import re
import shutil
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
OUT = ROOT / "content" / "releases"
VERSION = re.compile(r"^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$")
HEADING = re.compile(r"^## \[([^\]]+)\] - (\d{4}-\d{2}-\d{2})$")


def key(v):
    """Semantic version order; a pre-release comes before its release."""
    m = VERSION.match(v)
    pre = m.group(4)
    parts = [] if pre is None else [(0, int(p)) if p.isdigit() else (1, p) for p in pre.split(".")]
    return (int(m.group(1)), int(m.group(2)), int(m.group(3)), pre is None, parts)


def read(folder):
    v = folder.name
    notes = (folder / "RELEASE-NOTES.md").read_text(encoding="utf-8").strip().splitlines()
    m = HEADING.match(notes[0]) if notes else None
    if not m or m.group(1) != v[1:]:
        sys.exit(f"{folder}/RELEASE-NOTES.md must start with \"## [{v[1:]}] - YYYY-MM-DD\"")
    files = []
    sums = folder / "SHA256SUMS"
    if sums.exists():
        for line in sums.read_text(encoding="utf-8").splitlines():
            if line.strip():
                files.append(line.split(None, 1)[1].strip())
        files += ["SHA256SUMS", "SHA256SUMS.bundle"]
    return {"version": v, "date": m.group(2), "notes": "\n".join(notes[1:]).strip(), "files": files}


def support(r, releases):
    """What the release gets in security fixes, after SECURITY.md's
    "Supported versions": only the latest minor version gets them."""
    m = VERSION.match(r["version"])
    major, minor, pre = int(m.group(1)), int(m.group(2)), m.group(4)
    final = [x for x in releases if VERSION.match(x["version"]).group(4) is None]
    if pre is not None:
        return f"{r['version']} is a pre-release and gets no security fixes."
    latest = max(final, key=lambda x: key(x["version"]))
    lm = VERSION.match(latest["version"])
    lmajor, lminor = int(lm.group(1)), int(lm.group(2))
    series = f"v{major}.{minor}"
    if (major, minor) == (lmajor, lminor):
        text = (f"{series} is the latest minor version, so it gets security fixes, as patch releases "
                f"(`{series}.Z`), until the next minor version is released.")
        if latest["version"] != r["version"]:
            text += f" The newest of them is [{latest['version']}](/releases/{latest['version']}/)."
        return text
    return (f"{series} no longer gets security fixes, since v{lmajor}.{lminor} is the latest minor version: "
            f"upgrade to [{latest['version']}](/releases/{latest['version']}/).")


def main():
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    src = pathlib.Path(sys.argv[1])
    folders = [f for f in src.iterdir() if f.is_dir() and VERSION.match(f.name) and (f / "RELEASE-NOTES.md").exists()] \
        if src.is_dir() else []
    releases = sorted((read(f) for f in folders), key=lambda r: key(r["version"]), reverse=True)

    shutil.rmtree(OUT, ignore_errors=True)
    OUT.mkdir(parents=True)
    newest = f"The newest is [{releases[0]['version']}](/releases/{releases[0]['version']}/). " if releases else ""
    (OUT / "_index.md").write_text(
        "---\ntitle: Releases\n"
        "description: Every version of Alpheus, newest first, with its release notes, files and security fixes.\n"
        "---\n\n"
        + ("No version is released yet. What is on `main` is in [Changes](/docs/changes/).\n" if not releases else
           f"Every version of Alpheus, newest first. {newest}Each page has the release's notes, from "
           "[the changelog](/docs/changes/), and its files, which [Verify a release](/docs/verify/) shows "
           "how to check. Only the latest minor version gets security fixes; see [Security](/security/).\n"),
        encoding="utf-8")
    for i, r in enumerate(releases):
        files = ""
        if r["files"]:
            rows = "\n".join(f"- [`{f}`]({f})" for f in r["files"])
            files = (f"\n\n## Files\n\nThe files are signed and listed with their checksums in `SHA256SUMS`; "
                     f"[Verify a release](/docs/verify/) shows how to check them.\n\n{rows}")
        (OUT / f"{r['version']}.md").write_text(
            f"---\ntitle: \"Alpheus {r['version']}\"\n"
            f"description: \"Release notes, files and security fixes of Alpheus {r['version']}, released {r['date']}.\"\n"
            f"date: {r['date']}\nlastmod: {r['date']}\nweight: {i + 1}\n"
            f"url: /releases/{r['version']}/\n---\n\n"
            f"Released {r['date']}.\n\n"
            f"## Release notes\n\n{r['notes']}\n\n"
            f"## Security fixes\n\n{support(r, releases)} Report a vulnerability as [Security](/security/) says."
            f"{files}\n",
            encoding="utf-8")
    print(f"made {len(releases)} release pages from {src}")


if __name__ == "__main__":
    main()
