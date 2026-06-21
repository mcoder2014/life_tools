#!/usr/bin/env python3
"""Backward-compatible entry point for the video_subtitle package."""

from pathlib import Path as _Path
import sys as _sys

_PACKAGE_PARENT = str(_Path(__file__).resolve().parent.parent)
if _PACKAGE_PARENT not in _sys.path:
    _sys.path.insert(0, _PACKAGE_PARENT)

from video_subtitle.lib.asr import *
from video_subtitle.lib.audio import *
from video_subtitle.lib.cli import main, parse_args, run
from video_subtitle.lib.config import *
from video_subtitle.lib.context import *
from video_subtitle.lib.embedded_subtitles import *
from video_subtitle.lib.io_utils import *
from video_subtitle.lib.llm import *
from video_subtitle.lib.pipeline import *
from video_subtitle.lib.prompts import *
from video_subtitle.lib.progress import *
from video_subtitle.lib.runtime import *
from video_subtitle.lib.subtitle import *
from video_subtitle.lib.tos_client import *

if __name__ == "__main__":
    import sys
    sys.exit(main())
