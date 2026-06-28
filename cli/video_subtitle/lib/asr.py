import json
import time

import requests

from .config import ASR_QUERY_URL, ASR_SUBMIT_URL


def asr_headers(config, task_id, include_sequence):
    asr = config["asr"]
    headers = {
        "Content-Type": "application/json",
        "X-Api-Resource-Id": asr.get("resource_id", "volc.seedasr.auc"),
        "X-Api-Request-Id": task_id,
    }
    if asr.get("api_key"):
        headers["X-Api-Key"] = asr["api_key"]
    else:
        headers["X-Api-App-Key"] = asr.get("app_key", "")
        headers["X-Api-Access-Key"] = asr.get("access_key", "")
    if include_sequence:
        headers["X-Api-Sequence"] = "-1"
    return headers


def assert_asr_auth_config(config):
    asr = config["asr"]
    if asr.get("api_key"):
        return
    if asr.get("app_key") and asr.get("access_key"):
        return
    raise RuntimeError("missing config: asr.api_key or asr.app_key + asr.access_key")


def submit_asr_task(audio_url, source_language, config, task_id):
    assert_asr_auth_config(config)
    audio_config = config["audio"]
    body = {
        "user": {"uid": "life_tools"},
        "audio": {
            "format": audio_config.get("format", "mp3"),
            "url": audio_url,
            "language": source_language,
        },
        "request": {
            "model_name": config["asr"].get("model_name", "bigmodel"),
            "enable_itn": True,
            "enable_punc": True,
            "show_utterances": True,
        },
    }
    resp = requests.post(
        config["asr"].get("submit_url", ASR_SUBMIT_URL),
        headers=asr_headers(config, task_id, include_sequence=True),
        data=json.dumps(body),
        timeout=30,
    )
    check_asr_status(resp, expected_ok=True)


def check_asr_status(resp, expected_ok):
    code = resp.headers.get("X-Api-Status-Code", "")
    message = resp.headers.get("X-Api-Message", "")
    logid = resp.headers.get("X-Tt-Logid", "")
    if expected_ok and code != "20000000":
        raise RuntimeError("asr request failed: code=%s message=%s logid=%s" % (code, message, logid))
    return code


def query_asr_result(config, task_id):
    assert_asr_auth_config(config)
    resp = requests.post(
        config["asr"].get("query_url", ASR_QUERY_URL),
        headers=asr_headers(config, task_id, include_sequence=False),
        data="{}",
        timeout=30,
    )
    code = resp.headers.get("X-Api-Status-Code", "")
    if code in ("20000001", "20000002"):
        return None
    check_asr_status(resp, expected_ok=True)
    if not resp.text.strip():
        raise RuntimeError("asr query succeeded but response body is empty")
    return resp.json()


def wait_for_asr_result(config, task_id):
    deadline = time.monotonic() + int(config["asr"].get("max_wait_seconds", 7200))
    interval = int(config["asr"].get("poll_interval_seconds", 5))
    while True:
        result = query_asr_result(config, task_id)
        if result is not None:
            return result
        if time.monotonic() >= deadline:
            raise TimeoutError("asr task timed out: %s" % task_id)
        time.sleep(interval)


def normalize_words(item):
    words = []
    for word in item.get("words") or []:
        if not isinstance(word, dict):
            continue
        if "start_time" not in word or "end_time" not in word or "text" not in word:
            continue
        text = str(word["text"])
        if not text.strip():
            continue
        start_ms = int(word["start_time"])
        end_ms = int(word["end_time"])
        if start_ms < 0 or end_ms <= start_ms:
            continue
        words.append({
            "start_ms": start_ms,
            "end_ms": end_ms,
            "text": text,
        })
    return words


def extract_utterances(payload):
    result = payload.get("result")
    if isinstance(result, list):
        result = result[0] if result else {}
    if not isinstance(result, dict):
        raise ValueError("asr result is missing")

    utterances = result.get("utterances")
    if not utterances:
        raise ValueError("asr result has no utterances; submit request must enable show_utterances")

    normalized = []
    for index, item in enumerate(utterances, start=1):
        for key in ("start_time", "end_time", "text"):
            if key not in item:
                raise ValueError("utterance %d missing %s" % (index, key))
        normalized_item = {
            "id": index,
            "start_ms": int(item["start_time"]),
            "end_ms": int(item["end_time"]),
            "text": str(item["text"]).strip(),
        }
        words = normalize_words(item)
        if words:
            normalized_item["words"] = words
        normalized.append(normalized_item)
    return normalized
