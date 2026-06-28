import hashlib
import re
import shutil
from pathlib import Path


def choose_output_path(video_path, explicit_output):
    if explicit_output:
        return Path(explicit_output)

    video_path = Path(video_path)
    stem_path = video_path.with_suffix("")
    candidate = stem_path.with_name(stem_path.name + ".zh-CN.srt")
    if not candidate.exists():
        return candidate

    for i in range(1, 101):
        candidate = stem_path.with_name("%s.zh-CN_%d.srt" % (stem_path.name, i))
        if not candidate.exists():
            return candidate
    raise RuntimeError("no available subtitle filename after trying suffix _1 to _100")


def ensure_tool(name):
    if not shutil.which(name):
        raise RuntimeError("required command not found: %s" % name)


def safe_work_name(video_path):
    digest = hashlib.sha1(str(Path(video_path).resolve()).encode("utf-8")).hexdigest()[:10]
    safe_stem = re.sub(r"[^A-Za-z0-9._-]+", "_", Path(video_path).stem).strip("._")
    return (safe_stem or "video") + "_" + digest


def work_dir_for(video_path):
    return Path(video_path).parent / ".video_subtitle_work" / safe_work_name(video_path)
