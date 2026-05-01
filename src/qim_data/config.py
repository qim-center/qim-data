"""Configuration model for qim-data."""

from dataclasses import dataclass

from .constants import DEFAULT_APP_ID, DEFAULT_RELAY_URL, DEFAULT_TRANSIT_HELPER


@dataclass(frozen=True)
class QimDataConfig:
    relay_url: str = DEFAULT_RELAY_URL
    transit_helper: str = DEFAULT_TRANSIT_HELPER
    app_id: str = DEFAULT_APP_ID


def load_config() -> QimDataConfig:
    """Load runtime config.

    v0: returns built-in defaults.
    """
    return QimDataConfig()
