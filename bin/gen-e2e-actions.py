#! /usr/bin/python3
import sys
from pathlib import Path
import os

USAGE_TEXT = ""

def generate_file_content(test_name):
    return f"name: e2etest-{test_name}\n" + """
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  workflow_dispatch:


env:
  BREV_SETUP_TEST_CMD_DIR: /home/brev/workspace/brev-cli/actions-runner/_work/brev-cli/brev-cli

jobs:
  ci:
    runs-on: [self-hosted]
    defaults:
      run:
        shell: bash
    steps:
      - uses: actions/checkout@v2

      - uses: actions/setup-go@v2
        with:
          go-version: 1.18
      # - name: Force GitHub Actions' docker daemon to use vfs.
      #   run: |
      #     sudo systemctl stop docker
      #     echo '{"cgroup-parent":"/actions_job","storage-driver":"vfs"}' | sudo tee /etc/docker/daemon.json
      #     sudo systemctl start docker
      - name: test

        run: """ + f"go test -timeout 240s -run ^{test_name}$ github.com/brevdev/brev-cli/e2etest/setup\n"


def create_file_if_not_exist(path):
    if not os.path.exists(path):
        with open(path, "w") as f:
            f.write("")



if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(USAGE_TEXT)
        sys.exit(1)

    file_path_prefix = [".github", "workflows"]
    Path("/".join(file_path_prefix)).mkdir(parents=True, exist_ok=True)


    test_names = (name for name in sys.argv[1:] if name != "" )
    for test_name in test_names:
        path = Path("/".join(file_path_prefix + [test_name + ".yml"]))
        create_file_if_not_exist(path)
        with open(path, "w") as f:
            f.write(generate_file_content(test_name))
        print(f"Generated e2e-{test_name}.yml")
