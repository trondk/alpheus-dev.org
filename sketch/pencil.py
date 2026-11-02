"""Pencil-sketch drawing for the site's sketches: lines drawn twice with a
little jitter, on lined notebook paper, as inline SVG in the site's font.
A drawing script calls begin, draws with the functions here and prints
svg(...). The same seed gives the same drawing.
"""
import math, random

out = []
J = 1.6  # jitter of a pencil line, in px

def j(v, a=J):
    return v + random.uniform(-a, a)

def stroke(d, w=1.6, op=0.85, dash=None):
    extra = f' stroke-dasharray="{dash}"' if dash else ''
    out.append(f'<path d="{d}" stroke-width="{w}" opacity="{op}"{extra}/>')

def line(x1, y1, x2, y2, twice=True, w=1.6, dash=None):
    for k in range(2 if twice else 1):
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        L = math.hypot(x2 - x1, y2 - y1) or 1
        bow = random.uniform(-0.012, 0.012) * L
        nx, ny = -(y2 - y1) / L, (x2 - x1) / L
        stroke(f'M{j(x1-2):.1f},{j(y1):.1f} Q{mx+nx*bow:.1f},{my+ny*bow:.1f} {j(x2+2):.1f},{j(y2):.1f}',
               w if k == 0 else w * 0.7, 0.85 if k == 0 else 0.45, dash)

def rect(x, y, bw, h, **kw):
    line(x, y, x + bw, y, **kw); line(x + bw, y, x + bw, y + h, **kw)
    line(x + bw, y + h, x, y + h, **kw); line(x, y + h, x, y, **kw)

def ellipse(cx, cy, rx, ry, w=1.5):
    for k in range(2):
        pts = []
        start = random.uniform(0, math.tau)
        n = 28
        for i in range(n + 3):
            t = start + i / n * math.tau * (1.04 if k else 1.06)
            r = 1 + random.uniform(-0.025, 0.025)
            pts.append((cx + rx * r * math.cos(t), cy + ry * r * math.sin(t)))
        d = f'M{pts[0][0]:.1f},{pts[0][1]:.1f} ' + ' '.join(
            f'T{x:.1f},{y:.1f}' for x, y in pts[1:])
        stroke(d, w if k == 0 else w * 0.6, 0.8 if k == 0 else 0.4)

def text(x, y, s, size=20, anchor='middle', rot=None, weight='normal'):
    r = rot if rot is not None else random.uniform(-1.5, 1.5)
    out.append(f'<text fill="#2a2a2e" stroke="none" x="{x}" y="{y}" font-size="{size}" text-anchor="{anchor}" '
               f'font-weight="{weight}" transform="rotate({r:.1f} {x} {y})">{s}</text>')

def lines(x, y, rows, size=19, gap=24, **kw):
    for i, s in enumerate(rows):
        text(x, y + i * gap, s, size, rot=kw.get('rot', 0), anchor=kw.get('anchor', 'middle'))

def bubble(cx, cy, rx, ry, rows, tail, size=18):
    ellipse(cx, cy, rx, ry)
    tx, ty = tail
    # three small circles from the bubble's edge toward the tail
    ang = math.atan2(ty - cy, tx - cx)
    ex, ey = cx + rx * math.cos(ang), cy + ry * math.sin(ang)
    for i, r in enumerate((6.5, 4.5, 2.6)):
        f = (i + 1) / 4
        ellipse(ex + (tx - ex) * f, ey + (ty - ey) * f, r, r, w=1.1)
    n = len(rows)
    lines(cx, cy - (n - 1) * (size + 5) / 2 + size * 0.35, rows, size, size + 5, rot=random.uniform(-2, 2))

def arrow(x1, y1, x2, y2, dash=None, w=1.6):
    line(x1, y1, x2, y2, dash=dash, w=w)
    a = math.atan2(y2 - y1, x2 - x1)
    for s in (0.45, -0.45):
        line(x2, y2, x2 - 13 * math.cos(a + s), y2 - 13 * math.sin(a + s), twice=False, w=w)

def chunk(x, y, label):
    w, h, f = 34, 42, 10
    line(x, y, x + w - f, y, twice=False); line(x + w - f, y, x + w, y + f, twice=False)
    line(x + w, y + f, x + w, y + h, twice=False); line(x + w, y + h, x, y + h, twice=False)
    line(x, y + h, x, y, twice=False)
    line(x + w - f, y, x + w - f, y + f, twice=False, w=1); line(x + w - f, y + f, x + w, y + f, twice=False, w=1)
    for k in range(3):
        line(x + 6, y + 18 + k * 7, x + w - 7, y + 18 + k * 7, twice=False, w=0.8)
    text(x + w / 2, y + h + 18, label, 15)

def wave(y, amp, x0=40, x1=1160, w=1.8):
    for k in range(2):
        pts = [f'M{x0},{j(y)}']
        x = x0
        while x < x1:
            x += 60
            pts.append(f'Q{x-30:.0f},{j(y - amp if (x // 60) % 2 else y + amp)} {x},{j(y)}')
        stroke(' '.join(pts), w if k == 0 else w * 0.6, 0.75 if k == 0 else 0.4)

def hatch(x, y, w, h, step=9):
    for i in range(0, int(w + h), step):
        x1, y1 = x + i, y
        x2, y2 = x + i - h, y + h
        if x1 > x + w: y1 = y + (x1 - x - w); x1 = x + w
        if x2 < x: y2 = y + h - (x - x2); x2 = x
        stroke(f'M{x1:.1f},{y1:.1f} L{x2:.1f},{y2:.1f}', 0.7, 0.25)


def begin(seed):
    """Starts a drawing: clears it and seeds the jitter."""
    out.clear()
    random.seed(seed)


def svg(name, w, h, title, desc):
    """The drawing as an inline SVG with role img, a title and a
    description; name keeps its ids apart from other drawings on a page."""
    head = f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {w} {h}" role="img" aria-labelledby="{name}-title {name}-desc">
<title id="{name}-title">{title}</title>
<desc id="{name}-desc">{desc}</desc>
<defs>
  <filter id="{name}-pencil" x="-5%" y="-5%" width="110%" height="110%">
    <feTurbulence type="fractalNoise" baseFrequency="0.035" numOctaves="2" seed="3" result="n"/>
    <feDisplacementMap in="SourceGraphic" in2="n" scale="2.6" xChannelSelector="R" yChannelSelector="G" result="d"/>
    <feTurbulence type="fractalNoise" baseFrequency="0.9" numOctaves="1" seed="9" result="g"/>
    <feColorMatrix in="g" type="matrix" values="0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 -1.6 1.5" result="ga"/>
    <feComposite in="d" in2="ga" operator="in"/>
  </filter>
  <filter id="{name}-paper"><feTurbulence type="fractalNoise" baseFrequency="0.6" numOctaves="3" seed="2"/>
    <feColorMatrix values="0 0 0 0 0.45  0 0 0 0 0.42  0 0 0 0 0.35  0 0 0 0.06 0"/></filter>
</defs>
<rect width="{w}" height="{h}" fill="#fbf8ef"/>
<rect width="{w}" height="{h}" filter="url(#{name}-paper)"/>
'''
    paper = [f'<line x1="0" y1="{yy}" x2="{w}" y2="{yy}" stroke="#9fb8d6" stroke-width="0.8" opacity="0.45"/>' for yy in range(96, h, 32)]
    paper.append(f'<line x1="58" y1="0" x2="58" y2="{h}" stroke="#d98c8c" stroke-width="1.2" opacity="0.55"/>')
    return (head + '\n'.join(paper) + '\n<g class="ink" fill="none" stroke="#2f2f33" stroke-linecap="round" '
            f'stroke-linejoin="round" filter="url(#{name}-pencil)">\n' + '\n'.join(out) + '\n</g>\n</svg>\n')
