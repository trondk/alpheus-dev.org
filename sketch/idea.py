"""Draws the first idea of Alpheus as a pencil sketch on notebook paper.

Writes assets/idea-sketch.svg, which the idea page shows inline with the
sketch shortcode, so its text is in the site's own font (WWW-11).
The same seed gives the same drawing.

Usage: python3 sketch/idea.py > assets/idea-sketch.svg
"""
import os, sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from pencil import *  # noqa: E402,F403

begin(7)
W, H = 1000, 800
desc = ("A pencil sketch on notebook paper. Four steps, fetch, split, clean "
        "and aggregate, stand in a row above the ground, each a box with an "
        "agent beside the component, and split drawn three times for three "
        "replicas. Dashed arrows between them carry tiny signals such as "
        "“chunk 17 ready”. Under the ground runs a river: object "
        "storage, with chunk files and a manifest in it. Arrows go from each "
        "step down into the river and up from it into the next step. Thought "
        "bubbles say: every step runs at the same time; no message broker, "
        "only tiny signals, and even those are optional; a chunk is just a "
        "file and Alpheus never opens it; crash, outage or restart, pick up "
        "at the last ack; one StreamRun is one run: create, watch, delete.")

# --- Drawing --------------------------------------------------------------
text(84, 58, 'Alpheus — the first idea', 36, anchor='start', rot=-1.2, weight='bold')
wave(70, 3, 84, 520, w=1.4)
text(960, 54, '2 Nov 2026', 19, anchor='end', rot=1)

# the steps, above the ground
steps = [(84, 'fetch'), (316, 'split'), (548, 'clean'), (780, 'aggregate')]
BY, BW, BH = 280, 150, 96
for x, name in steps:
    if name == 'split':  # its replicas, behind it
        rect(x + 14, BY - 14, BW, BH, w=1.2)
        rect(x + 7, BY - 7, BW, BH, w=1.3)
    rect(x, BY, BW, BH)
    text(x + (BW - 36) / 2, BY + 46, name, 25 if len(name) < 9 else 21, weight='bold')
    text(x + (BW - 36) / 2, BY + 74, 'any language', 16)
    rect(x + BW - 34, BY + 10, 26, BH - 20, w=1.2)  # the agent beside it
    text(x + BW - 17, BY + BH / 2 + 4, 'agent', 13, rot=-90)
text(316 + BW + 30, BY - 18, '×3', 22)

# tiny signals from step to step
for (x, _), (nx, _) in zip(steps, steps[1:]):
    arrow(x + BW + 6, BY + 34, nx - 6, BY + 34, dash='4 6', w=1.1)
text(84 + BW + 41, BY + 24, '“chunk 17', 13)
text(84 + BW + 41, BY + 56, 'ready”', 13)

# the ground, and the river under it
GY = 420
line(30, GY, 970, GY, w=1.2)
for gx in range(46, 970, 36):
    line(gx, GY + 1, gx - 11, GY + 12, twice=False, w=0.8)
text(400, GY - 12, 'each chunk to one replica', 14, anchor='start')
wave(490, 8, 30, 970)
wave(625, 8, 30, 970)
text(960, 660, 'object storage (S3): the river runs underground', 18, anchor='end', rot=-0.8)
for i, cx in enumerate((100, 250, 440, 600, 760)):
    chunk(cx, 518, f'chunk {i+1}')
mx, my = 852, 512
rect(mx, my, 96, 78, w=1.3)
text(mx + 48, my + 22, 'manifest', 16, weight='bold')
for k in range(3):
    line(mx + 22, my + 36 + k * 13, mx + 84, my + 36 + k * 13, twice=False, w=0.8)
    text(mx + 8, my + 41 + k * 13, '✓', 12, anchor='start', rot=0)

# files go down into the river and come up in the next step
for i, (x, _) in enumerate(steps):
    cx = x + 50
    if i < 3:
        arrow(cx, BY + BH + 6, cx + 26, 512)
    if i > 0:
        arrow(cx - 64, 512, cx - 26, BY + BH + 6)

# thought bubbles
bubble(250, 165, 165, 56, ['every step runs', 'at the same time!'], (190, BY - 6), 23)
bubble(690, 160, 200, 64, ['no message broker:', 'only tiny signals, and', 'even those are optional'], (742, BY + 20), 19)
bubble(185, 735, 150, 46, ['a chunk is just a file.', 'Alpheus never opens it'], (200, 600), 18)
bubble(500, 738, 150, 44, ['crash, outage, restart?', 'pick up at the last ack'], (470, 640), 18)
bubble(815, 735, 150, 46, ['one StreamRun = one run:', 'create, watch, delete'], (860, 670), 18)

sys.stdout.write(svg("idea", W, H, "Alpheus, the first idea, as a pencil sketch", desc))
