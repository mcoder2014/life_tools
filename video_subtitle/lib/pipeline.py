import hashlib
import json
import uuid
from pathlib import Path

from .asr import extract_utterances, submit_asr_task, wait_for_asr_result
from .audio import extract_audio
from .context import build_background_context
from .embedded_subtitles import evaluate_embedded_candidates
from .io_utils import read_json, write_json
from .llm import create_openai_client, evaluate_consistency_with_llm, run_llm_jobs, split_batch_with_llm, translate_utterances
from .progress import NullProgressLogger, ProgressLogger
from .subtitle import build_subtitle_units_from_word_ranges, build_srt, original_subtitle_units, write_subtitle
from .tos_client import tos_object_key, upload_audio_to_tos



def subtitle_units_signature(subtitle_units):
    material = [
        {
            "id": int(item["id"]),
            "start_ms": int(item["start_ms"]),
            "end_ms": int(item["end_ms"]),
            "text": str(item["text"]),
        }
        for item in subtitle_units
    ]
    payload = json.dumps(material, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def read_cached_translations(work_dir, subtitle_units):
    translated_json = Path(work_dir) / "translations.json"
    meta_json = Path(work_dir) / "translations.meta.json"
    if not translated_json.exists() or not meta_json.exists():
        return None
    meta = read_json(meta_json)
    if meta.get("subtitle_units_signature") != subtitle_units_signature(subtitle_units):
        return None
    raw = read_json(translated_json)
    return {int(k): v for k, v in raw.items()}


def write_translation_cache(work_dir, subtitle_units, translations):
    work_dir = Path(work_dir)
    write_json(work_dir / "translations.json", {str(k): v for k, v in translations.items()})
    write_json(work_dir / "translations.meta.json", {
        "subtitle_units_signature": subtitle_units_signature(subtitle_units),
        "count": len(subtitle_units),
    })

def source_preview(items):
    if not items:
        return ""
    first = str(items[0].get("text", ""))
    last = str(items[-1].get("text", ""))
    return first if first == last else first + " | " + last


def time_range(items):
    if not items:
        return ""
    return "%s-%s" % (int(items[0]["start_ms"]), int(items[-1]["end_ms"]))


def id_range(items):
    if not items:
        return ""
    return "%s-%s" % (items[0]["id"], items[-1]["id"])


def build_split_chunks(utterances, config):
    cfg = config.get("subtitle_split", {})
    max_window_ms = int(float(cfg.get("max_window_seconds", 60)) * 1000)
    max_words = int(cfg.get("max_words_per_chunk", 120) or 0)
    max_utterances = int(cfg.get("max_utterances_per_chunk", 8) or 0)
    eligible = [item for item in utterances if item.get("words")]
    chunks = []
    current = []

    def current_words(items):
        return sum(len(item.get("words") or []) for item in items)

    def flush():
        if not current:
            return
        chunks.append({
            "index": len(chunks) + 1,
            "items": list(current),
            "source_ids": [int(item["id"]) for item in current],
            "start_ms": int(current[0]["start_ms"]),
            "end_ms": int(current[-1]["end_ms"]),
            "word_count": current_words(current),
            "utterance_count": len(current),
        })
        current[:] = []

    for item in eligible:
        candidate = current + [item]
        too_many_utterances = bool(max_utterances) and len(candidate) > max_utterances
        too_many_words = bool(max_words) and current and current_words(candidate) > max_words
        too_long = bool(max_window_ms) and current and int(item["end_ms"]) - int(current[0]["start_ms"]) > max_window_ms
        if too_many_utterances or too_many_words or too_long:
            flush()
        current.append(item)
    flush()
    return chunks


def split_chunk_signature(chunk, config):
    cfg = config.get("subtitle_split", {})
    prompt_cfg = config.get("prompts", {})
    material = {
        "source_ids": chunk["source_ids"],
        "limits": {
            "max_window_seconds": cfg.get("max_window_seconds", 60),
            "max_words_per_chunk": cfg.get("max_words_per_chunk", 120),
            "max_utterances_per_chunk": cfg.get("max_utterances_per_chunk", 8),
        },
        "prompt": prompt_cfg.get("split_template", ""),
        "items": [
            {
                "id": int(item["id"]),
                "start_ms": int(item["start_ms"]),
                "end_ms": int(item["end_ms"]),
                "text": str(item["text"]),
                "words": [
                    {
                        "start_ms": int(word["start_ms"]),
                        "end_ms": int(word["end_ms"]),
                        "text": str(word.get("text", "")),
                    }
                    for word in item.get("words") or []
                ],
            }
            for item in chunk["items"]
        ],
    }
    payload = json.dumps(material, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()[:16]


def split_chunk_cache_path(work_dir, chunk, config):
    signature = split_chunk_signature(chunk, config)
    return Path(work_dir) / "split_chunks" / ("chunk_%03d_%s.json" % (chunk["index"], signature))


def original_units_for_chunk(chunk):
    return original_subtitle_units(chunk["items"])


def renumber_subtitle_units(units):
    renumbered = []
    for index, item in enumerate(sorted(units, key=lambda part: (int(part["start_ms"]), int(part["end_ms"]), int(part.get("source_id", part["id"])))), start=1):
        copied = dict(item)
        copied["id"] = index
        renumbered.append(copied)
    return renumbered

def utterances_have_invalid_words(utterances):
    for item in utterances:
        for word in item.get("words") or []:
            text = str(word.get("text", ""))
            start_ms = int(word.get("start_ms", -1))
            end_ms = int(word.get("end_ms", -1))
            if not text.strip() or start_ms < 0 or end_ms <= start_ms:
                return True
    return False


def load_or_run_asr(video_path, work_dir, source_language, config, force_asr, logger=None):
    asr_json = work_dir / "asr_result.json"
    utterances_json = work_dir / "utterances.json"
    logger = logger or NullProgressLogger()
    if asr_json.exists() and utterances_json.exists() and not force_asr:
        utterances = read_json(utterances_json)
        if any(item.get("words") for item in utterances) and not utterances_have_invalid_words(utterances):
            logger.event("asr", "cache_hit", status="cache", count=len(utterances))
            return utterances
        try:
            refreshed = extract_utterances(read_json(asr_json))
        except Exception:
            return utterances
        if any(item.get("words") for item in refreshed):
            write_json(utterances_json, refreshed)
            logger.event("asr", "cache_refresh_words", status="ok", count=len(refreshed))
            return refreshed
        logger.event("asr", "cache_hit_without_words", status="cache", count=len(utterances))
        return utterances

    audio_path = work_dir / ("audio." + config["audio"].get("format", "mp3"))
    if force_asr or not audio_path.exists():
        logger.event("audio", "extract_start", status="start")
        extract_audio(video_path, audio_path, config)
        logger.event("audio", "extract_done", status="ok")
    else:
        logger.event("audio", "cache_hit", status="cache")

    object_key = tos_object_key(video_path, audio_path, config)
    logger.event("tos", "upload_start", status="start", object_key=object_key)
    signed_url = upload_audio_to_tos(audio_path, object_key, config)
    logger.event("tos", "upload_done", status="ok", object_key=object_key)
    task_id = str(uuid.uuid4())
    logger.event("asr", "submit_start", status="start")
    submit_asr_task(signed_url, source_language, config, task_id)
    logger.event("asr", "wait_start", status="start", task_id=task_id)
    result = wait_for_asr_result(config, task_id)
    utterances = extract_utterances(result)
    logger.event("asr", "done", status="ok", count=len(utterances))
    write_json(work_dir / "tos_object.json", {"object_key": object_key, "task_id": task_id})
    write_json(asr_json, result)
    write_json(utterances_json, utterances)
    return utterances


def candidate_report_for_cache(report):
    candidates = []
    for item in report.get("candidates", []):
        cached = dict(item)
        cached.pop("units", None)
        candidates.append(cached)
    selected = report.get("selected")
    selected_index = None
    if selected is not None:
        for idx, item in enumerate(report.get("candidates", [])):
            if item is selected:
                selected_index = idx
                break
    return {"selected_index": selected_index, "candidates": candidates}


def make_subtitle_units_from_embedded(video_path, utterances, work_dir, config, force_units, logger=None):
    report_json = work_dir / "embedded_candidates.json"
    units_json = work_dir / "subtitle_units.json"
    logger = logger or NullProgressLogger()
    if units_json.exists() and not force_units:
        units = read_json(units_json)
        logger.event("embedded", "cache_hit", status="cache", count=len(units))
        return units

    logger.event("embedded", "evaluate_start", status="start")
    report = evaluate_embedded_candidates(
        video_path,
        original_subtitle_units(utterances),
        work_dir,
        config,
        evaluate_consistency_with_llm,
    )
    write_json(report_json, candidate_report_for_cache(report))
    selected = report.get("selected")
    logger.event("embedded", "evaluate_done", status="ok", count=len(report.get("candidates", [])), selected=bool(selected))
    if not selected:
        return None
    units = []
    for index, item in enumerate(selected["units"], start=1):
        units.append({
            "id": index,
            "source": "embedded",
            "start_ms": int(item["start_ms"]),
            "end_ms": int(item["end_ms"]),
            "text": str(item["text"]),
        })
    write_json(units_json, units)
    return units


def should_attempt_split(utterances, config):
    cfg = config.get("subtitle_split", {})
    if not cfg.get("enabled", True):
        return False
    return any(item.get("words") for item in utterances)


def split_utterances_with_llm(utterances, config, work_dir, force_split=False, logger=None):
    logger = logger or NullProgressLogger()
    if not should_attempt_split(utterances, config):
        logger.event("llm_split", "skip", status="cache", count=len(utterances))
        return original_subtitle_units(utterances)
    chunks = build_split_chunks(utterances, config)
    if not chunks:
        return original_subtitle_units(utterances)
    cfg = config.get("subtitle_split", {})
    failure_policy = str(cfg.get("failure_policy", "fallback_chunk"))
    cache_enabled = bool(cfg.get("chunk_cache_enabled", True))
    client = create_openai_client(config)
    llm = config["llm"]
    raw_response_dir = work_dir / "split_raw_responses"
    jobs = []
    results_by_index = {}
    split_source_ids = set()
    logger.event("llm_split", "plan", status="start", count=len(chunks))

    for chunk in chunks:
        cache_path = split_chunk_cache_path(work_dir, chunk, config)
        if cache_enabled and cache_path.exists() and not force_split:
            cached = read_json(cache_path)
            results_by_index[chunk["index"]] = cached["units"]
            split_source_ids.update(int(item["id"]) for item in chunk["items"])
            logger.event("llm_split", "chunk_done", status="cache", index=chunk["index"], total=len(chunks), count=len(cached["units"]), cache="hit")
        else:
            jobs.append({"label": "split_%03d" % chunk["index"], "chunk": chunk})

    def worker(job, attempt):
        chunk = job["chunk"]
        logger.event(
            "llm_split",
            "chunk_start",
            status="start",
            index=chunk["index"],
            total=len(chunks),
            id_range=id_range(chunk["items"]),
            time_range=time_range(chunk["items"]),
            count=chunk["utterance_count"],
            word_count=chunk["word_count"],
            attempt=attempt,
            preview=source_preview(chunk["items"]),
        )
        ranges = split_batch_with_llm(client, llm, chunk["items"], config, raw_response_dir=raw_response_dir, batch_label=job["label"])
        chunk_units = build_subtitle_units_from_word_ranges(chunk["items"], ranges)
        if cache_enabled:
            write_json(split_chunk_cache_path(work_dir, chunk, config), {
                "source_ids": chunk["source_ids"],
                "signature": split_chunk_signature(chunk, config),
                "units": chunk_units,
            })
        logger.event("llm_split", "chunk_done", status="ok", index=chunk["index"], total=len(chunks), count=len(chunk_units), cache="miss", attempt=attempt)
        return {"chunk": chunk, "units": chunk_units}

    failed_by_index = {}
    job_results = run_llm_jobs(jobs, worker, config, "llm_split", logger=logger, return_exceptions=failure_policy != "fail")
    for job, result in zip(jobs, job_results):
        chunk = job["chunk"]
        if isinstance(result, Exception):
            failed_by_index[chunk["index"]] = str(result)
            continue
        results_by_index[chunk["index"]] = result["units"]
        split_source_ids.update(int(item["id"]) for item in chunk["items"])

    units = []
    for chunk in chunks:
        if chunk["index"] in results_by_index:
            units.extend(results_by_index[chunk["index"]])
            split_source_ids.update(int(item["id"]) for item in chunk["items"])
            continue
        if failure_policy == "fallback_asr":
            return original_subtitle_units(utterances)
        write_json(raw_response_dir / ("split_%03d_error.json" % chunk["index"]), {
            "ids": chunk["source_ids"],
            "error": failed_by_index.get(chunk["index"], "split chunk failed"),
        })
        logger.event("llm_split", "chunk_fallback", status="fallback", index=chunk["index"], total=len(chunks), error=failed_by_index.get(chunk["index"], "split chunk failed"))
        units.extend(original_units_for_chunk(chunk))
        split_source_ids.update(int(item["id"]) for item in chunk["items"])

    for item in utterances:
        if int(item["id"]) not in split_source_ids:
            units.append({
                "id": int(item["id"]),
                "source_id": int(item["id"]),
                "start_ms": int(item["start_ms"]),
                "end_ms": int(item["end_ms"]),
                "text": str(item["text"]),
            })
    return renumber_subtitle_units(units)

def load_or_build_subtitle_units(video_path, utterances, work_dir, config, force_asr=False, force_split=False, logger=None):
    units_json = work_dir / "subtitle_units.json"
    force_units = force_asr or force_split
    if units_json.exists() and not force_units:
        units = read_json(units_json)
        if logger:
            logger.event("subtitle_units", "cache_hit", status="cache", count=len(units))
        return units

    units = None
    if config.get("embedded_subtitles", {}).get("enabled", True) and not force_split:
        units = make_subtitle_units_from_embedded(video_path, utterances, work_dir, config, force_units, logger=logger)
    if units is None:
        try:
            units = split_utterances_with_llm(utterances, config, work_dir, force_split=force_split, logger=logger)
        except Exception as e:
            write_json(work_dir / "subtitle_split_error.json", {"error": str(e)})
            units = original_subtitle_units(utterances)
        write_json(units_json, units)
    return units


def load_or_translate(video_path, subtitle_units, work_dir, config, force_translate, allow_search_prompt, logger=None):
    logger = logger or NullProgressLogger()
    if not force_translate:
        cached = read_cached_translations(work_dir, subtitle_units)
        if cached is not None:
            logger.event("translation", "cache_hit", status="cache", count=len(cached))
            return cached

    logger.event("translation", "start", status="start", count=len(subtitle_units))
    background = build_background_context(video_path, config, allow_search_prompt)
    from .llm import build_translation_messages
    messages = build_translation_messages(background, subtitle_units[: min(5, len(subtitle_units))], config=config)
    write_json(work_dir / "translation_prompt_preview.json", messages)
    translations = translate_utterances(
        subtitle_units,
        background,
        config,
        raw_response_dir=work_dir / "translation_raw_responses",
        logger=logger,
    )
    write_translation_cache(work_dir, subtitle_units, translations)
    logger.event("translation", "done", status="ok", count=len(translations))
    return translations


def generate_subtitle(video_path, output_path, source_language, config, force_asr, force_translate, allow_search_prompt, force_split=False):
    from .runtime import work_dir_for

    work_dir = work_dir_for(video_path)
    work_dir.mkdir(parents=True, exist_ok=True)
    logger = ProgressLogger(work_dir, config)
    logger.event("run", "start", status="start", input=str(video_path), output=str(output_path))
    utterances = load_or_run_asr(video_path, work_dir, source_language, config, force_asr, logger=logger)
    subtitle_units = load_or_build_subtitle_units(video_path, utterances, work_dir, config, force_asr=force_asr, force_split=force_split, logger=logger)
    translations = load_or_translate(
        video_path,
        subtitle_units,
        work_dir,
        config,
        force_translate,
        allow_search_prompt=allow_search_prompt,
        logger=logger,
    )
    write_subtitle(output_path, build_srt(subtitle_units, translations))
    logger.event("run", "done", status="ok", output=str(output_path), count=len(subtitle_units))
    return output_path
