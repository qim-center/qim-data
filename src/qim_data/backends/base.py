"""Backend interface for transfer operations."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Protocol


@dataclass(frozen=True)
class SendRequest:
    source: Path
    code_length: int | None = None


@dataclass(frozen=True)
class ReceiveRequest:
    code: str | None = None


class TransferBackend(Protocol):
    def send(self, request: SendRequest) -> int:
        """Run a send transfer. Returns process exit code."""

    def receive(self, request: ReceiveRequest) -> int:
        """Run a receive transfer. Returns process exit code."""
