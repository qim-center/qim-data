"""Command-line interface for qim-data."""

from __future__ import annotations

import argparse

from . import __version__
from .app import QimDataApp
from .backends import WormholeCliBackend
from .config import load_config


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="qim-data",
        description="QIM Center wrapper for magic-wormhole",
    )
    parser.add_argument("--version", action="version", version=f"%(prog)s {__version__}")

    subparsers = parser.add_subparsers(dest="command", required=True)

    send_parser = subparsers.add_parser("send", help="Send file or directory")
    send_parser.add_argument("source", help="File or directory to send")
    send_parser.add_argument("--code-length", type=int, default=None, help="Wormhole code length")

    receive_parser = subparsers.add_parser("receive", help="Receive file")
    receive_parser.add_argument("code", nargs="?", default=None, help="Wormhole code")

    subparsers.add_parser("config", help="Print effective configuration")

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    config = load_config()

    if args.command == "config":
        print(f"relay_url={config.relay_url}")
        print(f"transit_helper={config.transit_helper}")
        print(f"app_id={config.app_id}")
        return 0

    app = QimDataApp(backend=WormholeCliBackend(config=config))

    if args.command == "send":
        return app.send(source=args.source, code_length=args.code_length)

    if args.command == "receive":
        return app.receive(code=args.code)

    parser.error(f"Unsupported command: {args.command}")
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
