import argparse
import sys
from pathlib import Path

from .config import DEFAULT_CONFIG_PATH, load_config, validate_runtime_config
from .pipeline import generate_subtitle
from .runtime import choose_output_path, ensure_tool


def parse_args(argv):
    parser = argparse.ArgumentParser(description="Generate Chinese SRT subtitles for one video.")
    parser.add_argument("--input", required=True, help="input video file")
    parser.add_argument("--config", default=str(DEFAULT_CONFIG_PATH), help="config JSON path")
    parser.add_argument("--output", help="output SRT path")
    parser.add_argument("--source-language", default="ja-JP", help="ASR source language, default ja-JP")
    parser.add_argument("--yes", action="store_true", help="skip optional prompts and do not run TMDB search without local NFO")
    parser.add_argument("--force-asr", action="store_true", help="ignore cached ASR result")
    parser.add_argument("--force-translate", action="store_true", help="ignore cached translation result")
    parser.add_argument("--force-split", action="store_true", help="ignore cached subtitle split units and split chunk cache")
    return parser.parse_args(argv)


def run(argv):
    args = parse_args(argv)
    video_path = Path(args.input)
    if not video_path.is_file():
        raise RuntimeError("input video does not exist: %s" % video_path)
    ensure_tool("ffmpeg")
    ensure_tool("ffprobe")
    config = load_config(args.config)
    validate_runtime_config(config)
    output_path = choose_output_path(video_path, args.output)
    generate_subtitle(
        video_path,
        output_path,
        args.source_language,
        config,
        args.force_asr,
        args.force_translate,
        allow_search_prompt=not args.yes,
        force_split=args.force_split,
    )
    print("subtitle written: %s" % output_path)


def main():
    try:
        run(sys.argv[1:])
    except Exception as e:
        print("error: %s" % e, file=sys.stderr)
        return 1
    return 0
