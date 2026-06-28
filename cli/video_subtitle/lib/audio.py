import subprocess

from .runtime import ensure_tool


def extract_audio(video_path, audio_path, config):
    ensure_tool("ffmpeg")
    audio = config["audio"]
    cmd = [
        "ffmpeg",
        "-y",
        "-i",
        str(video_path),
        "-vn",
        "-ac",
        str(audio.get("channels", 1)),
        "-ar",
        str(audio.get("sample_rate", 16000)),
        "-b:a",
        str(audio.get("bitrate", "64k")),
        str(audio_path),
    ]
    subprocess.run(cmd, check=True)
    return audio_path
