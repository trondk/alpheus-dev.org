#!/usr/bin/env python3
"""Check the built site in public/ (WWW-25, WWW-26).

- No page loads a resource from another domain. Ordinary links are fine.
- The front page, with everything it can load, weighs under 150 KB.
"""
import os
import re
import sys
from html.parser import HTMLParser
from urllib.parse import urljoin, urlparse

PUBLIC = sys.argv[1] if len(sys.argv) > 1 else "public"
HOST = "alpheus-dev.org"
LIMIT = 150 * 1024

# Tags and attributes that make the browser fetch something.
RESOURCE_ATTRS = {
    "script": ["src"], "img": ["src", "srcset"], "iframe": ["src"],
    "source": ["src", "srcset"], "video": ["src", "poster"], "audio": ["src"],
    "embed": ["src"], "object": ["data"], "link": ["href"],
}
# <link> relations that are only references, not fetched resources.
LINK_REFERENCES = {"canonical", "alternate", "license", "author", "me"}


class Resources(HTMLParser):
    def __init__(self):
        super().__init__()
        self.urls = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "link" and set((attrs.get("rel") or "").split()) <= LINK_REFERENCES:
            return
        for name in RESOURCE_ATTRS.get(tag, []):
            value = attrs.get(name)
            if value:
                self.urls += [part.split()[0] for part in value.split(",")] if name == "srcset" else [value]


def external(url):
    host = urlparse(url).hostname
    return host is not None and host != HOST


def css_urls(path):
    css = open(path, encoding="utf-8").read()
    return [u.strip("'\"") for u in re.findall(r"url\(([^)]+)\)", css)]


def local_path(url, page_url):
    path = urlparse(urljoin(page_url, url)).path
    return os.path.join(PUBLIC, path.lstrip("/"))


failed = False

for root, _, files in os.walk(PUBLIC):
    for name in files:
        path = os.path.join(root, name)
        if name.endswith(".html"):
            parser = Resources()
            parser.feed(open(path, encoding="utf-8").read())
            urls = parser.urls
        elif name.endswith(".css"):
            urls = css_urls(path)
        else:
            continue
        for url in filter(external, urls):
            print(f"FAIL {path}: loads {url} from another domain")
            failed = True

# Weight of the front page: the HTML, everything it references, and every
# file its stylesheets reference. Counting every font is stricter than
# what a browser actually fetches.
index = os.path.join(PUBLIC, "index.html")
parser = Resources()
parser.feed(open(index, encoding="utf-8").read())
files = {index}
for url in parser.urls:
    files.add(local_path(url, "/"))
for css in [f for f in list(files) if f.endswith(".css")]:
    css_url = "/" + os.path.relpath(css, PUBLIC)
    files.update(local_path(u, css_url) for u in css_urls(css) if not u.startswith("data:"))

total = 0
for f in sorted(files):
    if not os.path.exists(f):
        print(f"FAIL front page references missing file {f}")
        failed = True
        continue
    total += os.path.getsize(f)
print(f"{'ok  ' if total < LIMIT else 'FAIL'} front page weighs {total / 1024:.1f} KB (limit {LIMIT // 1024} KB, {len(files)} files)")
failed |= total >= LIMIT

sys.exit(1 if failed else 0)
