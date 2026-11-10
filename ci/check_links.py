#!/usr/bin/env python3
"""Check that every link and resource on the built site in public/ leads
to a page or file of the site.

Usage: ci/check_links.py [public]

Links to other sites are not followed. A link from a release's page to a
file in its folder, /releases/vX.Y.Z/<file>, counts when the release's page
exists: the web server serves the release's files from that folder, and
they are not part of the build.
"""

import html.parser
import pathlib
import re
import sys
import urllib.parse

PUBLIC = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "public")
RELEASE_FILE = re.compile(r"^/releases/(v[^/]+)/.*[^/]$")


class Links(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []

    def handle_starttag(self, tag, attrs):
        for name, value in attrs:
            if value and name in ("href", "src"):
                self.links.append(value)


def exists(path):
    target = PUBLIC / path.lstrip("/")
    if path.endswith("/"):
        return (target / "index.html").is_file()
    return target.is_file() or (target / "index.html").is_file()


def main():
    broken = []
    pages = sorted(PUBLIC.rglob("*.html"))
    for page in pages:
        parser = Links()
        parser.feed(page.read_text(encoding="utf-8"))
        base = "/" + page.relative_to(PUBLIC).as_posix()
        for link in parser.links:
            u = urllib.parse.urlsplit(link)
            if u.scheme or u.netloc or not u.path:
                continue
            path = urllib.parse.unquote(urllib.parse.urljoin(base, u.path))
            if exists(path):
                continue
            m = RELEASE_FILE.match(path)
            if m and (PUBLIC / "releases" / m.group(1) / "index.html").is_file():
                continue
            broken.append(f"{base}: {link}")
    for b in broken:
        print("broken", b)
    print(f"{'FAIL' if broken else 'ok  '} {len(pages)} pages, {len(broken)} broken links")
    sys.exit(1 if broken else 0)


if __name__ == "__main__":
    main()
