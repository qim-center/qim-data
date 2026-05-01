"""CLI backend implemented by invoking the wormhole command."""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

try:
    import pty
except ImportError:  # Windows
    pty = None

from ..config import QimDataConfig
from .base import ReceiveRequest, SendRequest


class WormholeCliBackend:
    def __init__(self, config: QimDataConfig) -> None:
        self._config = config

    def send(self, request: SendRequest) -> int:
        cmd = self._base_cmd() + ["send", "--no-qr", str(request.source)]
        if request.code_length is not None:
            cmd.extend(["--code-length", str(request.code_length)])
        return self._run_send_with_filtered_output(cmd)

    def receive(self, request: ReceiveRequest) -> int:
        cmd = self._base_cmd() + ["receive"]
        code = request.code
        if not code:
            code = input("Enter transfer code: ").strip()
        if code:
            cmd.append(code)
        return subprocess.run(cmd, check=False).returncode

    def _base_cmd(self) -> list[str]:
        return [
            "wormhole",
            f"--relay-url={self._config.relay_url}",
            f"--transit-helper={self._config.transit_helper}",
            f"--appid={self._config.app_id}",
        ]

    def _run_send_with_filtered_output(self, cmd: list[str]) -> int:
        if pty is None:
            return self._run_send_with_pipe_filtered_output(cmd)

        master_fd, slave_fd = pty.openpty()
        process = subprocess.Popen(
            cmd,
            stdin=None,
            stdout=slave_fd,
            stderr=slave_fd,
            close_fds=True,
        )
        os.close(slave_fd)

        suppress_next_blank = False
        suppress_next_receive = False
        pending = ""

        def handle_token(token: str) -> tuple[bool, str]:
            nonlocal suppress_next_blank, suppress_next_receive

            stripped = token.strip()

            if stripped == "On the other computer, please run:":
                suppress_next_blank = True
                suppress_next_receive = True
                return True, ""

            if suppress_next_blank and stripped == "":
                suppress_next_blank = False
                return True, ""

            if suppress_next_receive and stripped.startswith("wormhole receive "):
                suppress_next_receive = False
                return True, ""

            if stripped.startswith("Wormhole code is:"):
                token = token.replace("Wormhole code is:", "Transfer code is:", 1)

            return False, token

        try:
            while True:
                try:
                    data = os.read(master_fd, 4096)
                except OSError:
                    break
                if not data:
                    break

                pending += data.decode("utf-8", errors="replace")

                while True:
                    split_idx = -1
                    sep = ""
                    for candidate in ("\n", "\r"):
                        idx = pending.find(candidate)
                        if idx != -1 and (split_idx == -1 or idx < split_idx):
                            split_idx = idx
                            sep = candidate

                    if split_idx == -1:
                        break

                    token = pending[: split_idx + 1]
                    pending = pending[split_idx + 1 :]
                    suppress, rewritten = handle_token(token)
                    if not suppress:
                        sys.stdout.write(rewritten)
                        sys.stdout.flush()

            if pending:
                _, rewritten = handle_token(pending)
                if rewritten:
                    sys.stdout.write(rewritten)
                    sys.stdout.flush()
        finally:
            os.close(master_fd)

        return process.wait()

    def _run_send_with_pipe_filtered_output(self, cmd: list[str]) -> int:
        process = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )

        suppress_next_blank = False
        suppress_next_receive = False

        assert process.stdout is not None
        for line in process.stdout:
            stripped = line.strip()

            if stripped == "On the other computer, please run:":
                suppress_next_blank = True
                suppress_next_receive = True
                continue

            if suppress_next_blank and stripped == "":
                suppress_next_blank = False
                continue

            if suppress_next_receive and stripped.startswith("wormhole receive "):
                suppress_next_receive = False
                continue

            if stripped.startswith("Wormhole code is:"):
                line = line.replace("Wormhole code is:", "Transfer code is:", 1)

            sys.stdout.write(line)
            sys.stdout.flush()

        return process.wait()
