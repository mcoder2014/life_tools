from concurrent.futures import FIRST_COMPLETED, ThreadPoolExecutor, wait
import json
import random
import re
import sys
import time
from pathlib import Path

from .config import DEFAULT_ARK_BASE_URL, require_non_empty
from .io_utils import chunked, write_json
from .prompts import configured_template, render_prompt_template


def strip_json_markdown_fence(content):
    content = content.strip()
    if not content.startswith("```"):
        return content
    lines = content.splitlines()
    if len(lines) >= 3 and lines[0].startswith("```") and lines[-1].strip() == "```":
        return "\n".join(lines[1:-1]).strip()
    return content


def extract_json_payload(content):
    content = strip_json_markdown_fence(content)
    decoder = json.JSONDecoder()
    try:
        data, end = decoder.raw_decode(content)
        if not content[end:].strip():
            return data
    except json.JSONDecodeError:
        pass

    for index, char in enumerate(content):
        if char not in "[{":
            continue
        try:
            data, _ = decoder.raw_decode(content[index:])
            return data
        except json.JSONDecodeError:
            continue
    raise ValueError("translation response is not valid JSON")


def parse_items_response(content, expected_ids=None, label="response"):
    data = extract_json_payload(content)
    if isinstance(data, dict) and "items" in data:
        data = data["items"]
    if not isinstance(data, list):
        raise ValueError("%s must be a JSON array or an object with items" % label)
    if expected_ids is not None:
        expected = set(int(i) for i in expected_ids)
        actual = set()
        for item in data:
            if not isinstance(item, dict) or "id" not in item:
                raise ValueError("%s item must contain id" % label)
            actual.add(int(item["id"]))
        missing = sorted(expected - actual)
        extra = sorted(actual - expected)
        if missing:
            raise ValueError("%s missing ids: %s" % (label, missing))
        if extra:
            raise ValueError("%s has unexpected ids: %s" % (label, extra))
    return data


def parse_translation_response(content, expected_ids):
    data = parse_items_response(content, expected_ids, "translation response")
    translations = {}
    for item in data:
        if not isinstance(item, dict) or "id" not in item or "text" not in item:
            raise ValueError("translation item must contain id and text")
        translations[int(item["id"])] = str(item["text"])
    return translations


def parse_split_response(content, expected_source_ids):
    data = parse_items_response(content, None, "split response")
    expected = set(int(i) for i in expected_source_ids)
    ranges = []
    for item in data:
        if not isinstance(item, dict):
            raise ValueError("split item must be an object")
        for key in ("source_id", "word_start", "word_end", "text"):
            if key not in item:
                raise ValueError("split item missing %s" % key)
        source_id = int(item["source_id"])
        if source_id not in expected:
            raise ValueError("split response has unexpected source_id: %s" % source_id)
        ranges.append({
            "source_id": source_id,
            "word_start": int(item["word_start"]),
            "word_end": int(item["word_end"]),
            "text": str(item["text"]),
        })
    return ranges


def normalize_response_format_mode(mode):
    mode = str(mode or "none").strip().lower()
    if mode in ("", "false", "off", "none", "disabled", "prompt", "prompt_only"):
        return "none"
    if mode in ("auto", "json_schema", "json_object"):
        return mode
    raise ValueError("unsupported llm.response_format: %s" % mode)


def response_format_candidates(mode):
    mode = normalize_response_format_mode(mode)
    if mode == "auto":
        return ["json_schema", "json_object", "none"]
    return [mode]


def schema_for_items(name, item_properties, required):
    return {
        "type": "json_schema",
        "json_schema": {
            "name": name,
            "strict": True,
            "schema": {
                "type": "object",
                "additionalProperties": False,
                "properties": {
                    "items": {
                        "type": "array",
                        "items": {
                            "type": "object",
                            "additionalProperties": False,
                            "properties": item_properties,
                            "required": required,
                        },
                    },
                },
                "required": ["items"],
            },
        },
    }


def build_response_format(mode, schema_name="subtitle_translation_batch"):
    mode = normalize_response_format_mode(mode)
    if mode == "auto":
        mode = "json_schema"
    if mode == "none":
        return None
    if mode == "json_object":
        return {"type": "json_object"}
    if mode != "json_schema":
        raise ValueError("unsupported llm.response_format: %s" % mode)
    if schema_name == "subtitle_split_batch":
        return schema_for_items(
            schema_name,
            {
                "source_id": {"type": "integer"},
                "word_start": {"type": "integer"},
                "word_end": {"type": "integer"},
                "text": {"type": "string"},
            },
            ["source_id", "word_start", "word_end", "text"],
        )
    if schema_name == "subtitle_consistency_batch":
        return schema_for_items(
            schema_name,
            {
                "id": {"type": "integer"},
                "consistent": {"type": "boolean"},
                "reason": {"type": "string"},
            },
            ["id", "consistent", "reason"],
        )
    return schema_for_items(
        schema_name,
        {
            "id": {"type": "integer"},
            "text": {"type": "string"},
        },
        ["id", "text"],
    )


def is_response_format_unsupported(error):
    message = str(error).lower()
    has_format = any(token in message for token in (
        "response_format",
        "json_schema",
        "json_object",
        "structured output",
        "结构化输出",
    ))
    has_unsupported = any(token in message for token in (
        "unsupported",
        "not support",
        "not supported",
        "invalid parameter",
        "unknown parameter",
        "unrecognized",
        "not implemented",
        "不支持",
    ))
    return has_format and has_unsupported


def create_openai_client(config):
    try:
        from openai import OpenAI
    except ImportError as e:
        raise RuntimeError("missing Python dependency: openai") from e
    llm = config["llm"]
    return OpenAI(api_key=llm["api_key"], base_url=llm.get("base_url", DEFAULT_ARK_BASE_URL))

def llm_parallel_config(config):
    llm = config.get("llm", {})
    return {
        "parallel_requests": max(1, int(llm.get("parallel_requests", 10) or 1)),
        "max_batch_retries": max(1, int(llm.get("max_batch_retries", 10) or 1)),
        "retry_base_delay_seconds": max(0.0, float(llm.get("retry_base_delay_seconds", 1.0) or 0)),
        "retry_max_delay_seconds": max(0.0, float(llm.get("retry_max_delay_seconds", 30.0) or 0)),
    }


def annealing_delay_seconds(attempt, config):
    cfg = llm_parallel_config(config)
    if attempt <= 1 or cfg["retry_base_delay_seconds"] <= 0:
        return 0.0
    delay = cfg["retry_base_delay_seconds"] * (2 ** min(attempt - 2, 6))
    delay = min(delay, cfg["retry_max_delay_seconds"])
    return delay * (0.75 + random.random() * 0.5)


def run_llm_jobs(jobs, worker, config, stage, logger=None, sleep_func=time.sleep, return_exceptions=False):
    if logger is None:
        from .progress import NullProgressLogger
        logger = NullProgressLogger()
    jobs = list(jobs)
    if not jobs:
        return []
    cfg = llm_parallel_config(config)
    max_workers = min(cfg["parallel_requests"], len(jobs))
    max_attempts = cfg["max_batch_retries"]
    results = [None] * len(jobs)
    next_index = 0
    active = {}

    def job_label(job, index):
        return str(job.get("label", index + 1)) if isinstance(job, dict) else str(index + 1)

    def submit(executor, index, attempt):
        job = jobs[index]
        label = job_label(job, index)
        delay = annealing_delay_seconds(attempt, config)
        if delay > 0:
            logger.event(stage, "retry_sleep", status="retry", index=index + 1, total=len(jobs), attempt=attempt, delay_seconds=round(delay, 3), label=label)
            sleep_func(delay)
        logger.event(stage, "job_start", status="start", index=index + 1, total=len(jobs), attempt=attempt, label=label)
        future = executor.submit(worker, job, attempt)
        active[future] = (index, attempt)

    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        while next_index < len(jobs) and len(active) < max_workers:
            submit(executor, next_index, 1)
            next_index += 1
        while active:
            done, _ = wait(active.keys(), return_when=FIRST_COMPLETED)
            for future in done:
                index, attempt = active.pop(future)
                label = job_label(jobs[index], index)
                try:
                    results[index] = future.result()
                    logger.event(stage, "job_done", status="ok", index=index + 1, total=len(jobs), attempt=attempt, label=label)
                except Exception as e:
                    logger.event(stage, "job_failed", status="error", index=index + 1, total=len(jobs), attempt=attempt, label=label, error=str(e))
                    if attempt >= max_attempts:
                        if not return_exceptions:
                            raise
                        results[index] = e
                        logger.event(stage, "job_give_up", status="error", index=index + 1, total=len(jobs), attempt=attempt, label=label, error=str(e))
                    else:
                        submit(executor, index, attempt + 1)
                        continue
                while next_index < len(jobs) and len(active) < max_workers:
                    submit(executor, next_index, 1)
                    next_index += 1
    return results


def request_chat_completion(client, llm, messages, preferred_format, schema_name):
    candidates = response_format_candidates(preferred_format)
    last_error = None
    for index, candidate in enumerate(candidates):
        response_format = build_response_format(candidate, schema_name=schema_name)
        request = {
            "model": llm["model"],
            "messages": messages,
            "temperature": float(llm.get("temperature", 0.2)),
        }
        if response_format is not None:
            request["response_format"] = response_format
        timeout = float(llm.get("request_timeout_seconds", 120) or 0)
        if timeout > 0:
            request["timeout"] = timeout
        max_tokens = int(llm.get("max_tokens", 0) or 0)
        if max_tokens > 0:
            request["max_tokens"] = max_tokens
        try:
            response = client.chat.completions.create(**request)
        except Exception as e:
            last_error = e
            if index != len(candidates) - 1 and is_response_format_unsupported(e):
                print("response_format %s unsupported, fallback to next mode" % candidate, file=sys.stderr)
                continue
            raise
        content = response.choices[0].message.content
        if not content:
            raise ValueError("llm response is empty")
        return content, candidate
    raise last_error


def build_translation_messages(background, utterance_chunk, config=None, structured_output=False):
    items = [{"id": u["id"], "text": u["text"]} for u in utterance_chunk]
    payload = {"items": items} if structured_output else items
    if config is not None:
        template = configured_template(config, "translation")
        user = render_prompt_template(template, {
            "background": background or "无",
            "items_json": json.dumps(payload, ensure_ascii=False),
        })
        system = "你是专业影视字幕翻译。必须只输出 JSON，不要输出 Markdown。"
        return [{"role": "system", "content": system}, {"role": "user", "content": user}]
    if structured_output:
        system = (
            "你是专业影视字幕翻译。把源语言台词翻译成自然、简洁的简体中文影视字幕。"
            "必须只输出 JSON 对象，格式固定为 {\"items\":[{\"id\":1,\"text\":\"译文\"}]}，不要输出 Markdown。"
            "必须保持每条 id 不变；不要改写、合并、拆分 id；不要输出时间轴。"
            "角色名、专有名词、世界观术语优先参考背景信息。"
        )
    else:
        system = (
            "你是专业影视字幕翻译。把源语言台词翻译成自然、简洁的简体中文影视字幕。"
            "必须保持每条 id 不变，只输出 JSON 数组，不输出 Markdown。"
            "不要改写、合并、拆分 id；不要输出时间轴。"
            "角色名、专有名词、世界观术语优先参考背景信息。"
        )
    user = "背景信息：\n%s\n\n待翻译 JSON：\n%s" % (
        background or "无",
        json.dumps(payload, ensure_ascii=False),
    )
    return [{"role": "system", "content": system}, {"role": "user", "content": user}]


def request_llm_translation_batch(client, llm, background, batch, preferred_format, config=None):
    candidates = response_format_candidates(preferred_format)
    last_error = None
    for index, candidate in enumerate(candidates):
        response_format = build_response_format(candidate)
        messages = build_translation_messages(background, batch, config=config, structured_output=response_format is not None)
        try:
            return request_chat_completion(client, llm, messages, candidate, "subtitle_translation_batch")
        except Exception as e:
            last_error = e
            if index != len(candidates) - 1 and is_response_format_unsupported(e):
                print("response_format %s unsupported, fallback to next mode" % candidate, file=sys.stderr)
                continue
            raise
    raise last_error


def write_raw_response(raw_response_dir, prefix, batch_label, ids, attempt, content, error):
    if raw_response_dir is None:
        return
    raw_response_dir = Path(raw_response_dir)
    raw_response_dir.mkdir(parents=True, exist_ok=True)
    label = re.sub(r"[^A-Za-z0-9_.-]+", "_", str(batch_label)).strip("_") or "batch"
    first = ids[0] if ids else "none"
    last = ids[-1] if ids else "none"
    filename = "%s_%s_attempt_%d_ids_%s_%s.json" % (prefix, label, attempt, first, last)
    write_json(raw_response_dir / filename, {
        "ids": ids,
        "attempt": attempt,
        "error": str(error),
        "content": content,
    })


def write_raw_translation_response(raw_response_dir, batch_label, batch, attempt, content, error):
    ids = [int(item["id"]) for item in batch]
    write_raw_response(raw_response_dir, "translation", batch_label, ids, attempt, content, error)


def translate_batch_with_retry(call_model, batch, max_attempts=2, raw_response_dir=None, batch_label="batch"):
    max_attempts = max(1, int(max_attempts))
    expected_ids = [item["id"] for item in batch]
    last_error = None
    for attempt in range(1, max_attempts + 1):
        content = call_model(batch)
        try:
            return parse_translation_response(content, expected_ids)
        except ValueError as e:
            last_error = e
            write_raw_translation_response(raw_response_dir, batch_label, batch, attempt, content, e)
    if len(batch) == 1:
        raise last_error
    middle = len(batch) // 2
    translations = translate_batch_with_retry(
        call_model,
        batch[:middle],
        max_attempts=max_attempts,
        raw_response_dir=raw_response_dir,
        batch_label="%s_a" % batch_label,
    )
    translations.update(translate_batch_with_retry(
        call_model,
        batch[middle:],
        max_attempts=max_attempts,
        raw_response_dir=raw_response_dir,
        batch_label="%s_b" % batch_label,
    ))
    return translations


def translate_utterances(utterances, background, config, raw_response_dir=None, logger=None):
    require_non_empty(config, ["llm.api_key", "llm.model"])
    llm = config["llm"]
    client = create_openai_client(config)
    batch_size = int(llm.get("batch_size", 40))
    max_attempts = int(llm.get("max_parse_retries", 2))
    format_state = {"mode": llm.get("response_format", "auto")}
    translations = {}
    batches = list(chunked(utterances, batch_size))
    if logger is None:
        from .progress import NullProgressLogger
        logger = NullProgressLogger()

    def call_model(call_batch):
        content, used_format = request_llm_translation_batch(
            client,
            llm,
            background,
            call_batch,
            format_state["mode"],
            config=config,
        )
        if normalize_response_format_mode(format_state["mode"]) == "auto":
            format_state["mode"] = used_format
        return content

    jobs = [
        {"label": "batch_%03d" % index, "index": index, "batch": batch}
        for index, batch in enumerate(batches, start=1)
    ]

    def worker(job, attempt):
        batch = job["batch"]
        logger.event(
            "translation",
            "batch_start",
            status="start",
            index=job["index"],
            total=len(batches),
            id_range="%s-%s" % (batch[0]["id"], batch[-1]["id"]),
            count=len(batch),
            attempt=attempt,
            preview=str(batch[0].get("text", "")),
        )
        result = translate_batch_with_retry(
            call_model,
            batch,
            max_attempts=max_attempts,
            raw_response_dir=raw_response_dir,
            batch_label=job["label"],
        )
        logger.event(
            "translation",
            "batch_done",
            status="ok",
            index=job["index"],
            total=len(batches),
            id_range="%s-%s" % (batch[0]["id"], batch[-1]["id"]),
            count=len(batch),
            attempt=attempt,
        )
        return result

    for result in run_llm_jobs(jobs, worker, config, "translation", logger=logger):
        translations.update(result)
    return translations


def build_split_messages(batch, config):
    items = []
    for item in batch:
        words = item.get("words") or []
        items.append({
            "id": item["id"],
            "text": item["text"],
            "duration_ms": int(item["end_ms"]) - int(item["start_ms"]),
            "words": [
                {"index": idx, "text": word.get("text", "")}
                for idx, word in enumerate(words)
            ],
        })
    template = configured_template(config, "split")
    split_cfg = config.get("subtitle_split", {})
    user = render_prompt_template(template, {
        "items_json": json.dumps({"items": items}, ensure_ascii=False),
        "target_min_seconds": str(split_cfg.get("target_min_seconds", 2.0)),
        "target_max_seconds": str(split_cfg.get("target_max_seconds", 4.0)),
    })
    system = "你是字幕切分器。必须只输出 JSON，不要输出 Markdown。"
    return [{"role": "system", "content": system}, {"role": "user", "content": user}]


def split_batch_with_llm(client, llm, batch, config, raw_response_dir=None, batch_label="split"):
    messages = build_split_messages(batch, config)
    content, _ = request_chat_completion(client, llm, messages, llm.get("response_format", "auto"), "subtitle_split_batch")
    return parse_split_response(content, [item["id"] for item in batch])


def build_consistency_messages(samples, config):
    template = configured_template(config, "consistency")
    user = render_prompt_template(template, {"items_json": json.dumps({"items": samples}, ensure_ascii=False)})
    system = "你是字幕一致性评估器。必须只输出 JSON，不要输出 Markdown。"
    return [{"role": "system", "content": system}, {"role": "user", "content": user}]


def evaluate_consistency_with_llm(samples, config):
    if not samples:
        return 0.0, []
    client = create_openai_client(config)
    llm = config["llm"]

    def worker(job, attempt):
        content, _ = request_chat_completion(
            client,
            llm,
            build_consistency_messages(job["samples"], config),
            llm.get("response_format", "auto"),
            "subtitle_consistency_batch",
        )
        return parse_items_response(content, [item["id"] for item in job["samples"]], "consistency response")

    data = run_llm_jobs(
        [{"label": "consistency", "samples": samples}],
        worker,
        config,
        "consistency",
    )[0]
    results = []
    for item in data:
        results.append({
            "id": int(item["id"]),
            "consistent": bool(item.get("consistent")),
            "reason": str(item.get("reason", "")),
        })
    if not results:
        return 0.0, []
    consistent = sum(1 for item in results if item["consistent"])
    return consistent / float(len(results)), results
