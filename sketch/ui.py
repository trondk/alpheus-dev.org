"""Draws how alpheus ui should look, as a pencil sketch on notebook paper:
the list of runs, one run, and one replica with its logs and a replay.

Writes assets/ui-sketch.svg, which a page shows inline with the sketch
shortcode, so its text is in the site's own font (WWW-11).

Usage: python3 sketch/ui.py > assets/ui-sketch.svg
"""
import os, sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from pencil import *  # noqa: E402,F403

begin(11)
W, H = 1000, 1750


def window(x, y, w, h, title, url):
    """A browser window: a frame, a title bar with three dots and the URL."""
    rect(x, y, w, h)
    line(x, y + 34, x + w, y + 34, twice=False, w=1.2)
    for i in range(3):
        ellipse(x + 18 + i * 16, y + 17, 4.5, 4.5, w=1)
    rect(x + 80, y + 8, 300, 19, w=0.9)
    text(x + 90, y + 23, url, 13, anchor='start', rot=0)
    text(x + w - 14, y + 24, title, 15, anchor='end', rot=0, weight='bold')


def scribble(x, y, w, n=1, gap=12):
    """Lines of text too small to read: a wavy pencil line."""
    for k in range(n):
        yy = y + k * gap
        d = f'M{x},{yy}'
        xx = x
        while xx < x + w:
            xx += 14
            d += f' q7,{random.uniform(-3, 3):.1f} 14,0'
        stroke(d, 0.9, 0.5)


def mark(x, y, kind):
    """A phase mark: ✓ succeeded, ✗ failed, ● running."""
    if kind == 'ok':
        line(x - 6, y - 2, x - 1, y + 4, twice=False, w=1.6); line(x - 1, y + 4, x + 8, y - 8, twice=False, w=1.6)
    elif kind == 'fail':
        line(x - 6, y - 7, x + 6, y + 5, twice=False, w=1.8); line(x + 6, y - 7, x - 6, y + 5, twice=False, w=1.8)
    else:
        ellipse(x, y - 1, 5, 5, w=2)
        hatch(x - 4, y - 5, 8, 8, step=3)


text(84, 58, 'alpheus ui — how it should look', 34, anchor='start', rot=-1, weight='bold')
wave(70, 3, 84, 560, w=1.4)
text(960, 54, '2 Nov 2026', 19, anchor='end', rot=1)
text(84, 100, '$ alpheus ui     → opens localhost:7171 in your browser', 17, anchor='start', rot=0)

# --- 1. The runs -------------------------------------------------------------
X, Y, WW = 84, 130, 600
window(X, Y, WW, 300, '1  Runs', 'localhost:7171/')
text(X + 16, Y + 64, 'namespace: flows ▾', 15, anchor='start', rot=0)
for i, chip in enumerate(['all', 'running', 'failed']):
    rect(X + 210 + i * 78, Y + 48, 66, 24, w=1)
    text(X + 243 + i * 78, Y + 65, chip, 13, rot=0)
cols = [(16, 'RUN'), (150, 'PHASE'), (290, 'STEPS'), (360, 'STARTED'), (460, 'REASON')]
for cx, c in cols:
    text(X + cx, Y + 104, c, 12, anchor='start', rot=0, weight='bold')
line(X + 10, Y + 112, X + WW - 10, Y + 112, twice=False, w=0.9)
rows = [('log-report', 'running', 'Running', '2/3', '3m ago', ''),
        ('meter-etl', 'ok', 'Succeeded', '4/4', '1h ago', ''),
        ('sales', 'fail', 'Failed', '1/3', '20m ago', 'ComponentFailed'),
        ('nightly', 'ok', 'Partially', '3/3', '9h ago', 'FailedReplicas')]
for i, (name, kind, phase, steps, started, reason) in enumerate(rows):
    yy = Y + 140 + i * 36
    text(X + 16, yy, name, 16, anchor='start', rot=0, weight='bold' if name == 'sales' else 'normal')
    mark(X + 158, yy - 4, kind)
    text(X + 172, yy, phase, 15, anchor='start', rot=0)
    text(X + 290, yy, steps, 15, anchor='start', rot=0)
    text(X + 360, yy, started, 15, anchor='start', rot=0)
    text(X + 460, yy, reason, 14, anchor='start', rot=0)
rect(X + 8, Y + 140 + 2 * 36 - 22, WW - 16, 30, w=1.1, dash='3 5')
bubble(850, 230, 120, 70, ['live: it updates', 'as the status', 'changes'], (X + WW + 4, 240), 17)
arrow(X + 130, Y + 232, X + 120, Y + 360 + 30, w=1.3)
text(X + 140, Y + 330, 'click a run', 15, anchor='start')

# --- 2. One run ---------------------------------------------------------------
Y2 = 470
window(X, Y2, 832, 590, '2  Run sales', 'localhost:7171/runs/flows/sales')
mark(X + 24, Y2 + 70, 'fail')
text(X + 40, Y2 + 74, 'Failed: ComponentFailed', 20, anchor='start', rot=0, weight='bold')
text(X + 24, Y2 + 100, 'step filter replica 1 failed after 3 restarts: the component exited with code 1:', 14, anchor='start', rot=0)
text(X + 24, Y2 + 120, 'chunk 7 (source chunk 13) is one this component cannot handle', 14, anchor='start', rot=0)
# the flow as boxes in the river's language
GY2 = Y2 + 150
boxes = [(X + 40, 'generate', 'ok', 1), (X + 320, 'filter', 'fail', 2), (X + 600, 'count', 'running', 1)]
for bx, name, kind, reps in boxes:
    rect(bx, GY2, 170, 86)
    text(bx + 85, GY2 + 34, name, 21, weight='bold')
    mark(bx + 18, GY2 + 30, kind)
    for k in range(reps):
        ellipse(bx + 70 + k * 24, GY2 + 62, 7, 7, w=1.2)
        if name == 'filter' and k == 1:
            line(bx + 64 + k * 24, GY2 + 56, bx + 76 + k * 24, GY2 + 68, twice=False, w=1.4)
    text(bx + 85, GY2 + 102, f'{reps} replica' + ('s' if reps > 1 else ''), 13, rot=0)
for (ax, *_), (bx2, *_) in zip(boxes, boxes[1:]):
    arrow(ax + 176, GY2 + 43, bx2 - 6, GY2 + 43, w=1.4)
text(X + 265, GY2 + 30, '120 chunks', 13, rot=0)
text(X + 265, GY2 + 64, 'lag 2', 13, rot=0)
text(X + 545, GY2 + 30, '58 chunks', 13, rot=0)
text(X + 545, GY2 + 64, 'lag 0', 13, rot=0)
wave(GY2 + 130, 4, X + 20, X + 812, w=1.2)
text(X + 812, GY2 + 152, 'S3: chunks committed in manifests', 13, anchor='end', rot=0)
# replicas
TY = GY2 + 186
for cx, c in [(24, 'STEP'), (110, 'REPLICA'), (190, 'PHASE'), (290, 'ATTEMPT'), (370, 'REASON'), (520, 'MESSAGE')]:
    text(X + cx, TY, c, 12, anchor='start', rot=0, weight='bold')
line(X + 16, TY + 8, X + 816, TY + 8, twice=False, w=0.9)
reps = [('generate', '0', 'Succeeded', '1', '', ''), ('filter', '0', 'Running', '1', '', 'read 61 chunks'),
        ('filter', '1', 'Failed', '4', 'ComponentFailed', 'the component exited…'), ('count', '0', 'Running', '1', '', '')]
for i, row in enumerate(reps):
    yy = TY + 32 + i * 26
    for (cx, _), v in zip([(24, 0), (110, 0), (190, 0), (290, 0), (370, 0), (520, 0)], row):
        text(X + cx, yy, v, 14, anchor='start', rot=0, weight='bold' if row[1] == '1' and row[0] == 'filter' else 'normal')
rect(X + 16, TY + 32 + 2 * 26 - 19, 800, 26, w=1.1, dash='3 5')
# events
EY = TY + 150
text(X + 24, EY, 'EVENTS', 12, anchor='start', rot=0, weight='bold')
evs = ['10:02  Started  started 3 steps', '10:04  ComponentFailed  filter replica 1 attempt 1 failed, restarting (1 of 3)',
       '10:09  ComponentFailed  filter replica 1 attempt 4 failed after 3 restarts', '10:09  ComponentFailed  the run failed']
for i, ev in enumerate(evs):
    text(X + 24, EY + 24 + i * 21, ev, 13, anchor='start', rot=0)
arrow(X + 600, TY + 36 + 2 * 26, X + 470, Y2 + 600, w=1.3)
text(X + 610, TY + 112, 'click a replica', 15, anchor='start')

# --- 3. One replica: logs, error log, replay ---------------------------------------
Y3 = 1100
window(X, Y3, 832, 470, '3  filter, replica 1', 'localhost:7171/runs/flows/sales/filter/1')
for i, tab in enumerate(['Counts', 'Error log', 'Logs', 'Failures']):
    rect(X + 20 + i * 110, Y3 + 50, 100, 28, w=1.4 if tab == 'Logs' else 1)
    text(X + 70 + i * 110, Y3 + 69, tab, 15, rot=0, weight='bold' if tab == 'Logs' else 'normal')
text(X + 480, Y3 + 69, 'attempt 4 ▾', 15, anchor='start', rot=0)
text(X + 600, Y3 + 69, 'component ▾', 15, anchor='start', rot=0)
rect(X + 20, Y3 + 92, 792, 220, w=1.1)
logs = ['{"level":"DEBUG","msg":"read chunk","n":6,"header_number":11}',
        '{"level":"DEBUG","msg":"read chunk","n":7,"header_number":13}',
        '{"level":"ERROR","msg":"component failed","error":"chunk 7 (source',
        '  chunk 13) is one this component cannot handle","exit_code":1}']
for i, l in enumerate(logs):
    text(X + 34, Y3 + 120 + i * 22, l, 13, anchor='start', rot=0)
scribble(X + 34, Y3 + 220, 500, 3, 18)
text(X + 34, Y3 + 300, 'kept in S3 when the attempt failed, so the pod can be gone', 13, anchor='start', rot=0)
rect(X + 20, Y3 + 332, 280, 44, w=1.8)
text(X + 160, Y3 + 360, 'Replay attempt 4  ▸', 19, rot=0, weight='bold')
text(X + 320, Y3 + 350, 'runs: alpheus replay sales filter --replica 1', 14, anchor='start', rot=0)
text(X + 320, Y3 + 370, 'the same image, on the 2 chunks it had open, at debug level', 14, anchor='start', rot=0)
text(X + 20, Y3 + 420, 'Error log: code, message and input chunk of each entry, from results/…/_errors/', 14, anchor='start', rot=0)
text(X + 20, Y3 + 442, 'Counts: exact, committed numbers per unit, and the live metrics next to them', 14, anchor='start', rot=0)

bubble(250, 1650, 170, 58, ['read-only: it sees what', 'your kubeconfig lets you'], (250, Y3 + 470), 17)
bubble(700, 1660, 210, 56, ['one page in the CLI binary:', 'Go and plain HTML, nothing to install'], (650, Y3 + 470), 17)

desc = ("A pencil sketch of the alpheus ui, a page the CLI serves on localhost. "
        "Window 1, Runs: a namespace selector, filters for all, running and failed, and a table of runs "
        "with phase, steps finished, start and reason, where the failed run sales stands out. "
        "Window 2, Run sales: Failed with reason ComponentFailed and the message, the flow drawn as boxes "
        "generate, filter with two replicas, one crossed out, and count, with chunks and lag on the arrows, "
        "a table of replicas with phase, attempt, reason and message, and the run's events. "
        "Window 3, filter replica 1: tabs for counts, error log, logs and failures; the logs of attempt 4 "
        "show debug lines and the error, kept in S3, and a button Replay attempt 4 that runs alpheus replay "
        "on the chunks the attempt had open. Notes say the page is live, read-only with your kubeconfig, "
        "and part of the CLI binary.")
sys.stdout.write(svg("ui", W, H, "How alpheus ui should look, as a pencil sketch", desc))
