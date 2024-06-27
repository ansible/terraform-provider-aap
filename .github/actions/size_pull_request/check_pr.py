#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# (c) 2024 Aubin Bikouo <@abikouo>
# GNU General Public License v3.0+
#     (see https://www.gnu.org/licenses/gpl-3.0.txt)

from argparse import ArgumentParser
import subprocess
import re

def RunDiff(path: str) -> None:
    command = ["git diff --cached --stat $(git merge-base FETCH_HEAD origin)"]
    proc = subprocess.Popen(
        command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True, cwd=path
    )
    out, _ = proc.communicate()
    m = re.search('(\d*) files changed, (\d*) insertions\(\+\), (\d*) deletions\(\-\)', out.decode())
    if m:
        files = int(m.group(1))
        insertions = int(m.group(2))
        deletions = int(m.group(3))
        print(f"files = {files} - insertions = {insertions} - deletions = {deletions}")


if __name__ == '__main__':
    """Check PR size and push corresponding message and/or add label."""
    parser = ArgumentParser()
    parser.add_argument("--path", required=True, help="Path to the repository.")

    args = parser.parse_args()
    RunDiff(args.path)
