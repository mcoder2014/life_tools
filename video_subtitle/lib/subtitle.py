import re
from pathlib import Path


def format_srt_time(ms):
    ms = int(ms)
    hours = ms // 3600000
    ms %= 3600000
    minutes = ms // 60000
    ms %= 60000
    seconds = ms // 1000
    millis = ms % 1000
    return "%02d:%02d:%02d,%03d" % (hours, minutes, seconds, millis)


def build_srt(utterances, translations):
    blocks = []
    for item in utterances:
        item_id = item["id"]
        if item_id not in translations:
            raise ValueError("missing translation for id %s" % item_id)
        blocks.append(
            "%d\n%s --> %s\n%s" % (
                item_id,
                format_srt_time(item["start_ms"]),
                format_srt_time(item["end_ms"]),
                translations[item_id].strip(),
            )
        )
    return "\n\n".join(blocks) + "\n"


def write_subtitle(path, content):
    path = Path(path)
    if path.exists():
        raise RuntimeError("output already exists: %s" % path)
    path.write_text(content, encoding="utf-8")


def original_subtitle_units(utterances):
    units = []
    for item in utterances:
        unit = {
            "id": int(item["id"]),
            "source_id": int(item["id"]),
            "start_ms": int(item["start_ms"]),
            "end_ms": int(item["end_ms"]),
            "text": str(item["text"]),
        }
        units.append(unit)
    return units


def build_subtitle_units_from_word_ranges(utterances, ranges):
    by_id = {int(item["id"]): item for item in utterances}
    used = {}
    units = []
    for index, part in enumerate(ranges, start=1):
        source_id = int(part["source_id"])
        if source_id not in by_id:
            raise ValueError("subtitle split references unknown source_id: %s" % source_id)
        source = by_id[source_id]
        words = source.get("words") or []
        if not words:
            raise ValueError("subtitle split requires words for source_id: %s" % source_id)
        word_start = int(part["word_start"])
        word_end = int(part["word_end"])
        if word_start < 0 or word_end < word_start or word_end >= len(words):
            raise ValueError("subtitle split word range out of bounds for source_id: %s" % source_id)
        key = source_id
        previous = used.get(key, -1)
        if word_start <= previous:
            raise ValueError("subtitle split ranges overlap for source_id: %s" % source_id)
        if word_start != previous + 1:
            raise ValueError("subtitle split ranges leave a gap for source_id: %s" % source_id)
        used[key] = word_end
        text = str(part.get("text") or "").strip()
        if not text:
            text = "".join(str(w.get("text", "")) for w in words[word_start:word_end + 1]).strip()
        if not text:
            raise ValueError("subtitle split produced empty text for source_id: %s" % source_id)
        units.append({
            "id": index,
            "source_id": source_id,
            "start_ms": int(words[word_start]["start_ms"]),
            "end_ms": int(words[word_end]["end_ms"]),
            "text": text,
        })
    for source_id, source in by_id.items():
        if source.get("words") and used.get(source_id, -1) != len(source["words"]) - 1:
            raise ValueError("subtitle split ranges do not cover all words for source_id: %s" % source_id)
    return units


def parse_srt_time(value):
    h, m, rest = value.split(":")
    s, ms = rest.split(",")
    return ((int(h) * 3600 + int(m) * 60 + int(s)) * 1000) + int(ms)


SRT_BLOCK_RE = re.compile(
    r"(?ms)^(\d+)\n"
    r"(\d\d:\d\d:\d\d,\d{3}) --> (\d\d:\d\d:\d\d,\d{3})\n"
    r"(.*?)(?=\n\n\d+\n\d\d:\d\d:\d\d,\d{3} --> |\Z)"
)


def parse_srt_units(path):
    text = Path(path).read_text(encoding="utf-8-sig")
    units = []
    for index, (_, start, end, body) in enumerate(SRT_BLOCK_RE.findall(text), start=1):
        clean = body.strip()
        if not clean:
            continue
        units.append({
            "id": index,
            "start_ms": parse_srt_time(start),
            "end_ms": parse_srt_time(end),
            "text": clean,
        })
    return units
