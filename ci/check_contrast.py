#!/usr/bin/env python3
"""Check the palette in main.css against WCAG 2.1 AA (WWW-9, WWW-41).

Reads the colour tokens for the light theme (:root) and the dark theme
(:root inside prefers-color-scheme: dark) and checks every combination
the theme uses. Exits non-zero if any combination fails.
"""
import re
import sys

CSS = sys.argv[1] if len(sys.argv) > 1 else "themes/alpheus/assets/css/main.css"

TEXT = ["text", "groundwater", "spring", "sediment"]
# (foreground, background, minimum ratio, why)
PAIRS = [(fg, bg, 4.5, "text") for fg in TEXT for bg in ("bg", "surface")] + [
    ("horizon", "bg", 3.0, "graphics, WCAG 1.4.11"),
    ("spring", "bg", 3.0, "focus outline, WCAG 1.4.11"),
]


def tokens(block):
    return dict(re.findall(r"--([a-z-]+):\s*(#[0-9A-Fa-f]{6})", block))


def luminance(hex_colour):
    channels = [int(hex_colour[i:i + 2], 16) / 255 for i in (1, 3, 5)]
    r, g, b = [c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4 for c in channels]
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(a, b):
    la, lb = sorted((luminance(a), luminance(b)), reverse=True)
    return (la + 0.05) / (lb + 0.05)


css = open(CSS, encoding="utf-8").read()
light_block = re.search(r"^:root\s*{([^}]*)}", css, re.M)
dark_block = re.search(r"prefers-color-scheme:\s*dark\)\s*{\s*:root\s*{([^}]*)}", css)
if not light_block or not dark_block:
    sys.exit(f"{CSS}: could not find the light and dark :root blocks")

light = tokens(light_block.group(1))
themes = {"light": light, "dark": {**light, **tokens(dark_block.group(1))}}

failed = 0
for theme, colours in themes.items():
    for fg, bg, minimum, why in PAIRS:
        value = ratio(colours[fg], colours[bg])
        ok = value >= minimum
        failed += not ok
        print(f"{'ok  ' if ok else 'FAIL'} {theme:5} {fg:11} on {bg:7} {value:5.2f}:1 (min {minimum}, {why})")

sys.exit(1 if failed else 0)
