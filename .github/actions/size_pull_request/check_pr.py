#!/usr/bin/env python3
# -*- coding: utf-8 -*-
# (c) 2024 Aubin Bikouo <@abikouo>
# GNU General Public License v3.0+
#     (see https://www.gnu.org/licenses/gpl-3.0.txt)

from argparse import ArgumentParser
import subprocess
import re
import requests
import os


def WriteComment(repository: str, pr_number: int, comment: str) -> None:
    url = f"https://api.github.com/repos/{repository}/issues/{pr_number}/comments"
    result = requests.post(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "Authorization": "Bearer %s" % os.environ.get("GITHUB_TOKEN"),
        },
        json={"body": comment},
    )
    if result.status_code != 200:
        raise RuntimeError(f"Post to URL {url} returned status code = {result.status_code}")


def RunDiff(path: str, repository: str, pr_number: int) -> None:
    command = ["git diff --cached --stat $(git merge-base FETCH_HEAD origin)"]
    proc = subprocess.Popen(
        command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, shell=True, cwd=path
    )
    out, _ = proc.communicate()
    WriteComment(
        repository,
        pr_number,
        f"Output => {out.decode()}",
    )
    m = re.search(
        "(\d*) files changed, (\d*) insertions\(\+\), (\d*) deletions\(\-\)",
        out.decode(),
    )
    if m:
        files = int(m.group(1))
        insertions = int(m.group(2))
        deletions = int(m.group(3))
        WriteComment(
            repository,
            pr_number,
            f"files = {files} - insertions = {insertions} - deletions = {deletions}",
        )


if __name__ == "__main__":
    """Check PR size and push corresponding message and/or add label."""
    parser = ArgumentParser()
    parser.add_argument("--path", required=True, help="Path to the repository.")
    parser.add_argument("--repository", required=True, help="Repository name org/name.")
    parser.add_argument(
        "--pr-number", type=int, required=True, help="The pull request number."
    )

    args = parser.parse_args()
    RunDiff(args.path, args.repository, args.pr_number)
