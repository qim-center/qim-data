from qim_data.cli import build_parser


def test_parser_accepts_send_command() -> None:
    parser = build_parser()
    args = parser.parse_args(["send", "file.dat"])
    assert args.command == "send"
    assert args.source == "file.dat"
