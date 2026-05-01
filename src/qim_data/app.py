"""Application orchestration for qim-data commands."""

from __future__ import annotations

from pathlib import Path

from .backends.base import ReceiveRequest, SendRequest, TransferBackend


class QimDataApp:
    def __init__(self, backend: TransferBackend) -> None:
        self._backend = backend

    def send(self, source: str, code_length: int | None = None) -> int:
        request = SendRequest(source=Path(source), code_length=code_length)
        return self._backend.send(request)

    def receive(self, code: str | None = None) -> int:
        request = ReceiveRequest(code=code)
        return self._backend.receive(request)
