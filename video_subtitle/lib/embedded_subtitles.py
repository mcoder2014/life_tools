import json
import re
import subprocess
import sys
from pathlib import Path

from .subtitle import parse_srt_units

TEXT_CODECS = {"ass", "ssa", "subrip", "srt", "webvtt", "mov_text", "text"}
IMAGE_CODECS = {"hdmv_pgs_subtitle", "dvd_subtitle", "xsub", "dvb_subtitle"}


def run(cmd):
    return subprocess.run(cmd, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def require_ok(result, label):
    if result.returncode != 0:
        detail = result.stderr.strip() or result.stdout.strip()
        raise RuntimeError("%s failed:\n%s" % (label, detail))
    return result.stdout


def probe_streams(video_path):
    cmd = [
        "ffprobe",
        "-hide_banner",
        "-v",
        "error",
        "-show_entries",
        "stream=index,codec_type,codec_name:stream_tags=language,title",
        "-of",
        "json",
        str(video_path),
    ]
    data = json.loads(require_ok(run(cmd), "ffprobe"))
    return data.get("streams", [])


def subtitle_streams(video_path):
    return [s for s in probe_streams(video_path) if s.get("codec_type") == "subtitle"]


def strip_style(text):
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = text.replace("\\N", "\n").replace("\\n", "\n")
    text = re.sub(r"\{\\[^{}]*\}", "", text)
    text = re.sub(r"</?[^>\n]+>", "", text)
    text = re.sub(r"[ \t]+\n", "\n", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def is_drawing_only(text):
    compact = " ".join(text.split()).lower()
    if not compact:
        return True
    if "m 0 0 l" in compact:
        return True
    return bool(re.fullmatch(r"[mnlbspc0-9 .-]+", compact)) and any(
        token in compact.split() for token in ("m", "l")
    )


def clean_srt_file(src, dst):
    from .subtitle import SRT_BLOCK_RE

    blocks = SRT_BLOCK_RE.findall(Path(src).read_text(encoding="utf-8-sig"))
    if not blocks:
        raise RuntimeError("no valid SRT blocks parsed: %s" % src)
    out = []
    skipped = []
    new_num = 1
    for old_num, start, end, body in blocks:
        text = strip_style(body)
        if is_drawing_only(text):
            skipped.append(int(old_num))
            continue
        out.extend([str(new_num), "%s --> %s" % (start, end), text, ""])
        new_num += 1
    Path(dst).write_text("\n".join(out).rstrip() + "\n", encoding="utf-8")
    return new_num - 1, skipped


def extract_subtitle_stream(video_path, stream, work_dir):
    codec = stream.get("codec_name") or "subtitle"
    if codec in IMAGE_CODECS or codec not in TEXT_CODECS:
        return None
    work_dir = Path(work_dir)
    work_dir.mkdir(parents=True, exist_ok=True)
    idx = stream["index"]
    raw_ext = "ass" if codec in {"ass", "ssa"} else "srt"
    raw_path = work_dir / ("embedded_%s.source.%s" % (idx, raw_ext))
    srt_path = work_dir / ("embedded_%s.source.srt" % idx)
    clean_path = work_dir / ("embedded_%s.clean.srt" % idx)
    if codec in {"ass", "ssa"}:
        require_ok(run(["ffmpeg", "-hide_banner", "-y", "-i", str(video_path), "-map", "0:%s" % idx, "-c:s", "copy", str(raw_path)]), "ffmpeg subtitle extract")
        require_ok(run(["ffmpeg", "-hide_banner", "-y", "-i", str(raw_path), str(srt_path)]), "ffmpeg srt convert")
    else:
        require_ok(run(["ffmpeg", "-hide_banner", "-y", "-i", str(video_path), "-map", "0:%s" % idx, str(srt_path)]), "ffmpeg srt convert")
    count, skipped = clean_srt_file(srt_path, clean_path)
    return {
        "stream": stream,
        "path": str(clean_path),
        "cue_count": count,
        "skipped": skipped,
        "units": parse_srt_units(clean_path),
    }


def overlap_ms(a, b):
    return max(0, min(a["end_ms"], b["end_ms"]) - max(a["start_ms"], b["start_ms"]))


def align_candidate(asr_units, embedded_units):
    aligned = []
    used = set()
    for asr in asr_units:
        best = None
        best_overlap = 0
        for idx, emb in enumerate(embedded_units):
            if idx in used:
                continue
            overlap = overlap_ms(asr, emb)
            if overlap > best_overlap:
                best = (idx, emb)
                best_overlap = overlap
        if best is not None and best_overlap > 0:
            used.add(best[0])
            aligned.append({"asr": asr, "embedded": best[1], "overlap_ms": best_overlap})
    return aligned


def mechanical_score(asr_units, embedded_units):
    if not asr_units or not embedded_units:
        return {"time_coverage": 0.0, "aligned_ratio": 0.0, "aligned": []}
    aligned = align_candidate(asr_units, embedded_units)
    asr_duration = sum(max(0, int(u["end_ms"]) - int(u["start_ms"])) for u in asr_units)
    covered = sum(item["overlap_ms"] for item in aligned)
    return {
        "time_coverage": covered / float(asr_duration or 1),
        "aligned_ratio": len(aligned) / float(len(asr_units) or 1),
        "aligned": aligned,
    }


def consistency_samples(aligned, sample_size):
    if not aligned:
        return []
    sample_size = max(1, int(sample_size))
    if len(aligned) <= sample_size:
        selected = aligned
    else:
        step = max(1, len(aligned) // sample_size)
        selected = aligned[::step][:sample_size]
    samples = []
    for idx, item in enumerate(selected, start=1):
        samples.append({
            "id": idx,
            "asr_text": item["asr"].get("text", ""),
            "embedded_text": item["embedded"].get("text", ""),
        })
    return samples


def should_adopt_embedded_candidate(score, config):
    cfg = config.get("embedded_subtitles", {})
    return (
        float(score.get("time_coverage", 0)) >= float(cfg.get("min_time_coverage", 0.8))
        and float(score.get("aligned_ratio", 0)) >= float(cfg.get("min_aligned_ratio", 0.7))
        and float(score.get("llm_consistency", 0)) >= float(cfg.get("min_llm_consistency", 0.8))
    )


def evaluate_embedded_candidates(video_path, asr_units, work_dir, config, evaluate_llm):
    cfg = config.get("embedded_subtitles", {})
    if not cfg.get("enabled", True):
        return {"selected": None, "candidates": []}
    candidates = []
    for stream in subtitle_streams(video_path):
        try:
            extracted = extract_subtitle_stream(video_path, stream, Path(work_dir) / "embedded")
        except Exception as e:
            print("warning: embedded subtitle extract failed: %s" % e, file=sys.stderr)
            continue
        if not extracted:
            continue
        score = mechanical_score(asr_units, extracted["units"])
        if score["time_coverage"] >= float(cfg.get("min_time_coverage", 0.8)) and score["aligned_ratio"] >= float(cfg.get("min_aligned_ratio", 0.7)):
            samples = consistency_samples(score["aligned"], cfg.get("sample_size", 12))
            llm_score, llm_results = evaluate_llm(samples, config)
        else:
            samples = []
            llm_score = 0.0
            llm_results = []
        candidate = {
            "stream": stream,
            "path": extracted["path"],
            "cue_count": extracted["cue_count"],
            "time_coverage": score["time_coverage"],
            "aligned_ratio": score["aligned_ratio"],
            "llm_consistency": llm_score,
            "llm_samples": samples,
            "llm_results": llm_results,
            "adopt": should_adopt_embedded_candidate({
                "time_coverage": score["time_coverage"],
                "aligned_ratio": score["aligned_ratio"],
                "llm_consistency": llm_score,
            }, config),
            "units": extracted["units"],
        }
        candidates.append(candidate)
    candidates.sort(key=lambda item: (item["adopt"], item["llm_consistency"], item["time_coverage"], item["aligned_ratio"]), reverse=True)
    selected = candidates[0] if candidates and candidates[0]["adopt"] and cfg.get("auto_adopt", True) else None
    return {"selected": selected, "candidates": candidates}
