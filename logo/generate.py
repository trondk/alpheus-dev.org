# Draws the Alpheus logos and index.html, which shows them side by side:
# logo 4, the modern mark and name, with and without the motto, which the
# website uses, and logo 1, the roundel, for stickers. The website's copies
# and its favicon are written to the theme's static/ folder; animate.sh
# makes the GIFs.
import os, math
OUT = os.path.dirname(os.path.abspath(__file__))
GW, SP, HZ, LS, SK = "#0F4A55", "#1F6E6C", "#80857D", "#EEF0EE", "#23292D"
SP_L, GW_L = "#5FC2BD", "#7FB8C2"
FONT = "Spectral, 'Iowan Old Style', Georgia, serif"

def river(x, y, s, above, under, horizon, w=3.2, hw=1.4, span=52):
    """The river from the site: a line that dips under a horizon, runs
    dashed underground, rises again and ends in an arrow. Centered on x, y."""
    return f'''<g transform="translate({x} {y}) scale({s})" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <line x1="-{span}" y1="4" x2="{span}" y2="4" stroke="{horizon}" stroke-width="{hw}"/>
  <path d="M-50 -12 Q-42 -18 -34 -12 T-20 -12 C-13 -12 -16 8 -12 16" stroke="{above}" stroke-width="{w}"/>
  <path d="M-12 16 C-6 27 6 27 12 16" stroke="{under}" stroke-width="{w*0.8}" stroke-dasharray="{w*1.1} {w*1.1}"/>
  <path d="M12 16 C16 8 13 -12 20 -12 Q28 -6 36 -12 T48 -12" stroke="{above}" stroke-width="{w}"/>
  <path d="M42 -18 L50 -12 L42 -6" stroke="{above}" stroke-width="{w}"/>
</g>'''

def star(cx, cy, r, fill, points=5, inner=0.45, rot=-90):
    pts = []
    for i in range(points * 2):
        rr = r if i % 2 == 0 else r * inner
        a = math.radians(rot + i * 180 / points)
        pts.append(f"{cx + rr*math.cos(a):.2f},{cy + rr*math.sin(a):.2f}")
    return f'<polygon points="{" ".join(pts)}" fill="{fill}"/>'

def arc_text(id_, cx, cy, r, text, size, fill, top=True, spacing=3, weight=600):
    if top:
        d = f"M {cx-r} {cy} A {r} {r} 0 0 1 {cx+r} {cy}"
    else:
        d = f"M {cx-r} {cy} A {r} {r} 0 0 0 {cx+r} {cy}"
    return (f'<path id="{id_}" d="{d}" fill="none"/>'
            f'<text font-family="{FONT}" font-weight="{weight}" font-size="{size}" letter-spacing="{spacing}" fill="{fill}">'
            f'<textPath href="#{id_}" startOffset="50%" text-anchor="middle">{text}</textPath></text>')

def svg(name, vb, title, body):
    with open(os.path.join(OUT, name), "w") as f:
        f.write(f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="{vb}" role="img" aria-labelledby="t">
<title id="t">{title}</title>
{body}
</svg>
''')

# 1. Roundel: a round medal with the name on the upper arc and the motto
# on the lower, the river in the middle.
svg("1-roundel.svg", "-100 -100 200 200", "Alpheus: roundel", f'''
<circle r="97" fill="{GW}"/>
<circle r="90" fill="none" stroke="{LS}" stroke-width="1.6"/>
<circle r="60" fill="{LS}" stroke="{SP_L}" stroke-width="3"/>
{arc_text("r1top", 0, 0, 68, "ALPHEUS", 22, LS, True, 6)}
{arc_text("r1bot", 0, 0, 82, "PER SAXA FLUIT", 11.5, GW_L, False, 3)}
{star(-75, 0, 6, LS)}
{star(75, 0, 6, LS)}
{river(0, 0, 0.98, SP, GW, HZ)}
''')

# 4. Modern: a flat mark and a wordmark. The river, drawn bold, in a
# rounded square; the name in lower case next to it. The name follows the
# reader's light or dark theme; 4-modern-dark.svg is fixed for dark.
SANS = "'Atkinson Hyperlegible Next', 'Atkinson Hyperlegible', Inter, 'Helvetica Neue', Arial, sans-serif"

def ease(x):
    x = min(max(x, 0.0), 1.0)
    return x * x * (3 - 2 * x)

def drawn(t, start, end):
    """How much of a part is drawn at time t, when it is drawn from start to end."""
    return ease((t - start) / (end - start))

def mark(t=1.0):
    """The river in a rounded square. t from 0 to 1 draws it: the left part,
    then the underground part, the right part and last the arrow."""
    left, under, right, arrow = drawn(t, 0, .35), drawn(t, .35, .55), drawn(t, .55, .85), drawn(t, .85, 1)
    def solid(p):  # a path with pathLength 1, drawn up to p; round caps
        # would show a dot for nothing drawn, so hide it then
        hidden = ' visibility="hidden"' if p <= 0 else ""
        return f'pathLength="1" stroke-dasharray="1 1" stroke-dashoffset="{1 - p:.4f}"{hidden}'
    dashes, pos = [], 0.0  # the underground dashes, up to under
    while pos < under * 100:
        dashes += [min(13, under * 100 - pos), 9]
        pos += 22
    dashes += [0, 1000]
    return f'''<rect x="6" y="6" width="88" height="88" rx="22" fill="{GW}"/>
<g transform="translate(50 48) scale(0.66)" fill="none" stroke-linecap="round" stroke-linejoin="round">
  <line x1="-46" y1="4" x2="46" y2="4" stroke="{HZ}" stroke-width="3"/>
  <path d="M-50 -12 Q-42 -18 -34 -12 T-20 -12 C-13 -12 -16 8 -12 16" stroke="{SP_L}" stroke-width="7" {solid(left)}/>
  <path d="M-12 16 C-6 27 6 27 12 16" stroke="{GW_L}" stroke-width="5" stroke-linecap="butt" pathLength="100" stroke-dasharray="{' '.join(f'{d:.2f}' for d in dashes)}"/>
  <path d="M12 16 C16 8 13 -12 20 -12 Q28 -6 36 -12 T48 -12" stroke="{SP_L}" stroke-width="7" {solid(right)}/>
  <path d="M40 -20 L50 -12 L40 -4" stroke="{SP_L}" stroke-width="7" {solid(arrow)}/>
</g>'''

def words(motto):
    if not motto:
        return f'<text class="word" x="112" y="66" font-family="{SANS}" font-weight="700" font-size="50" letter-spacing="-1">alpheus</text>'
    return (f'<text class="word" x="112" y="58" font-family="{SANS}" font-weight="700" font-size="50" letter-spacing="-1">alpheus</text>\n'
            f'<text class="motto" x="115" y="86" font-family="{SANS}" font-weight="600" font-size="13.5" letter-spacing="4.2">PER SAXA FLUIT</text>')

LIGHT = f".word {{ fill: {GW}; }} .motto {{ fill: {SP}; }}"
DARK = f".word {{ fill: {LS}; }} .motto {{ fill: {SP_L}; }}"
for motto in (False, True):
    base = "4-modern-motto" if motto else "4-modern"
    what = "modern, with the motto" if motto else "modern"
    # The plain file follows the reader's theme; -dark is always for dark.
    svg(f"{base}.svg", "0 0 340 100", f"Alpheus: {what}",
        f"<style>{LIGHT} @media (prefers-color-scheme: dark) {{ {DARK} }}</style>\n{mark()}\n{words(motto)}")
    svg(f"{base}-dark.svg", "0 0 340 100", f"Alpheus: {what}, on dark",
        f"<style>{DARK}</style>\n{mark()}\n{words(motto)}")

# The website's copies: the logos and the mark alone as the favicon.
SITE = os.path.join(OUT, "..", "www", "site", "themes", "alpheus", "static")
os.makedirs(os.path.join(SITE, "logo"), exist_ok=True)
for name in ("4-modern.svg", "4-modern-dark.svg", "4-modern-motto.svg", "4-modern-motto-dark.svg"):
    with open(os.path.join(OUT, name)) as src, open(os.path.join(SITE, "logo", name), "w") as dst:
        dst.write(src.read())
with open(os.path.join(SITE, "favicon.svg"), "w") as f:
    f.write(f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="4 4 92 92">
{mark()}
</svg>
''')

def frames(folder, motto, dark, n=30):
    """Writes the frames of the animated logo: the river drawn in n steps, on
    a solid background, since a GIF has no partial transparency."""
    os.makedirs(folder, exist_ok=True)
    bg = SK if dark else LS
    for i in range(n + 1):
        with open(os.path.join(folder, f"{i:03d}.svg"), "w") as f:
            f.write(f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 340 100" width="680" height="200">
<style>{DARK if dark else LIGHT}</style>
<rect width="340" height="100" fill="{bg}"/>
{mark(i / n)}
{words(motto)}
</svg>
''')

cards = "".join(f'<figure><img src="{n}" alt="{t}"><figcaption>{t}</figcaption></figure>' for n, t in [
    ("1-roundel.svg", "1 · Roundel, for stickers"),
    ("4-modern.svg", "4 · Modern"), ("4-modern-motto.svg", "4 · Modern, with the motto")])
with open(os.path.join(OUT, "index.html"), "w") as f:
    f.write(f'''<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Alpheus logos</title>
<style>
body {{ margin: 0; font-family: Georgia, serif; color: {SK}; }}
section {{ padding: 24px; display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 24px; }}
section.light {{ background: {LS}; }} section.dark {{ background: {SK}; color: {LS}; }}
h2 {{ grid-column: 1 / -1; margin: 0; font-weight: 600; }}
figure {{ margin: 0; text-align: center; }}
img {{ width: 100%; height: 200px; object-fit: contain; }}
.small img {{ height: 48px; }}
</style></head><body>
<section class="light"><h2>On the light background</h2>{cards}</section>
<section class="dark"><h2>On the dark background</h2>{cards.replace("4-modern.svg", "4-modern-dark.svg").replace("4-modern-motto.svg", "4-modern-motto-dark.svg")}</section>
<section class="light small"><h2>Small, as in the site header</h2>{cards}</section>
<section class="light"><h2>Animated (for the README and slides, not the site)</h2>
<figure><img src="4-modern.gif" alt="Modern, animated"><figcaption>4-modern.gif</figcaption></figure>
<figure><img src="4-modern-motto.gif" alt="Modern with the motto, animated"><figcaption>4-modern-motto.gif</figcaption></figure></section>
<section class="dark"><h2>Animated, on dark</h2>
<figure><img src="4-modern-dark.gif" alt="Modern, animated, on dark"><figcaption>4-modern-dark.gif</figcaption></figure>
<figure><img src="4-modern-motto-dark.gif" alt="Modern with the motto, animated, on dark"><figcaption>4-modern-motto-dark.gif</figcaption></figure></section>
</body></html>
''')
print("ok")
