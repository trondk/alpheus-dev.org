# Copyright The Alpheus Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""count is the last step of the log-report example: it counts the lines of
each level in all of its input and writes the totals as CSV.
"""

import alpheus


def handle(inp, out):
    levels = {}

    def write_totals(out):
        with out.create("default", "csv") as f:
            f.write(b"level,lines\n")
            for level in sorted(levels):
                f.write(f"{level},{levels[level]}\n".encode())

    inp.on_eos(write_totals)  # before reading, so the agent knows
    for chunk in inp.chunks():
        with open(chunk.path, "rb") as src:
            for line in src:
                level = line.split(b" ", 1)[0].decode()
                levels[level] = levels.get(level, 0) + 1


if __name__ == "__main__":
    alpheus.run(handle)
