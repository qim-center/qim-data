"""CLI backend implemented by invoking the wormhole command."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path

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
        if request.code:
            cmd.append(request.code)
        return self._run_receive_with_filtered_output(cmd)

    def _base_cmd(self) -> list[str]:
        return [
            "wormhole",
            f"--relay-url={self._config.relay_url}",
            f"--transit-helper={self._config.transit_helper}",
            f"--appid={self._config.app_id}",
        ]

    def _run_send_with_filtered_output(self, cmd: list[str]) -> int:
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

    def _run_receive_with_filtered_output(self, cmd: list[str]) -> int:
        process = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )

        assert process.stdout is not None
        for line in process.stdout:
            line = line.replace("Enter receive wormhole code:", "Enter transfer code:")
            sys.stdout.write(line)
            sys.stdout.flush()

        return process.wait()
