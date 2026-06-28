import json
import os
from pathlib import Path

DEFAULT_CONFIG_PATH = Path("/etc/life_tools/video_subtitle.json")
DEFAULT_ARK_BASE_URL = "https://ark.cn-beijing.volces.com/api/coding/v3"
ASR_SUBMIT_URL = "https://openspeech.bytedance.com/api/v3/auc/bigmodel/submit"
ASR_QUERY_URL = "https://openspeech.bytedance.com/api/v3/auc/bigmodel/query"

ENV_OVERRIDES = {
    "VIDEO_SUBTITLE_ASR_API_KEY": ("asr", "api_key"),
    "VIDEO_SUBTITLE_ASR_APP_KEY": ("asr", "app_key"),
    "VIDEO_SUBTITLE_ASR_ACCESS_KEY": ("asr", "access_key"),
    "VIDEO_SUBTITLE_ASR_RESOURCE_ID": ("asr", "resource_id"),
    "VIDEO_SUBTITLE_LLM_API_KEY": ("llm", "api_key"),
    "VIDEO_SUBTITLE_LLM_BASE_URL": ("llm", "base_url"),
    "VIDEO_SUBTITLE_LLM_MODEL": ("llm", "model"),
    "VIDEO_SUBTITLE_TMDB_API_KEY": ("tmdb", "api_key"),
    "VIDEO_SUBTITLE_TMDB_BEARER_TOKEN": ("tmdb", "bearer_token"),
    "VIDEO_SUBTITLE_TOS_ACCESS_KEY": ("tos", "access_key"),
    "VIDEO_SUBTITLE_TOS_SECRET_KEY": ("tos", "secret_key"),
    "VIDEO_SUBTITLE_TOS_ENDPOINT": ("tos", "endpoint"),
    "VIDEO_SUBTITLE_TOS_REGION": ("tos", "region"),
    "VIDEO_SUBTITLE_TOS_BUCKET": ("tos", "bucket"),
    "VIDEO_SUBTITLE_TOS_PREFIX": ("tos", "prefix"),
}


def default_config():
    return {
        "asr": {
            "submit_url": ASR_SUBMIT_URL,
            "query_url": ASR_QUERY_URL,
            "api_key": "",
            "app_key": "",
            "access_key": "",
            "resource_id": "volc.seedasr.auc",
            "model_name": "bigmodel",
            "poll_interval_seconds": 5,
            "max_wait_seconds": 7200,
        },
        "audio": {
            "format": "mp3",
            "sample_rate": 16000,
            "channels": 1,
            "bitrate": "64k",
        },
        "llm": {
            "api_key": "",
            "base_url": DEFAULT_ARK_BASE_URL,
            "model": "",
            "temperature": 0.2,
            "batch_size": 40,
            "response_format": "auto",
            "max_parse_retries": 2,
            "request_timeout_seconds": 120,
            "parallel_requests": 10,
            "max_batch_retries": 10,
            "retry_base_delay_seconds": 1.0,
            "retry_max_delay_seconds": 30.0,
        },
        "prompts": {
            "translation_template": "",
            "split_template": "",
            "consistency_template": "",
        },
        "subtitle_split": {
            "enabled": True,
            "target_min_seconds": 2.0,
            "target_max_seconds": 4.0,
            "use_word_timing": True,
            "max_window_seconds": 60,
            "max_words_per_chunk": 120,
            "max_utterances_per_chunk": 8,
            "chunk_cache_enabled": True,
            "failure_policy": "fallback_chunk",
        },
        "logging": {
            "progress_jsonl": True,
            "stderr_preview_chars": 20,
        },
        "embedded_subtitles": {
            "enabled": True,
            "auto_adopt": True,
            "min_time_coverage": 0.8,
            "min_aligned_ratio": 0.7,
            "min_llm_consistency": 0.8,
            "sample_size": 12,
        },
        "tmdb": {
            "api_key": "",
            "bearer_token": "",
            "language": "zh-CN",
        },
        "tos": {
            "access_key": "",
            "secret_key": "",
            "endpoint": "",
            "region": "",
            "bucket": "",
            "prefix": "video-subtitle/audio",
            "presigned_url_ttl_seconds": 86400,
        },
    }


def merge_dict(base, override):
    for key, value in override.items():
        if isinstance(value, dict) and isinstance(base.get(key), dict):
            merge_dict(base[key], value)
        else:
            base[key] = value
    return base


def load_config(config_path=DEFAULT_CONFIG_PATH, env=None):
    env = env if env is not None else os.environ
    config = default_config()
    path = Path(config_path)
    if path.exists():
        with path.open("r", encoding="utf-8") as f:
            merge_dict(config, json.load(f))

    for env_name, keys in ENV_OVERRIDES.items():
        value = env.get(env_name)
        if value is None or value == "":
            continue
        section, name = keys
        config.setdefault(section, {})[name] = value
    return config


def require_non_empty(config, paths):
    missing = []
    for dotted in paths:
        cur = config
        for part in dotted.split("."):
            cur = cur.get(part, {}) if isinstance(cur, dict) else {}
        if cur in ("", None, {}):
            missing.append(dotted)
    if missing:
        raise RuntimeError("missing config: " + ", ".join(missing))


def validate_runtime_config(config):
    from .asr import assert_asr_auth_config

    require_non_empty(config, [
        "tos.access_key",
        "tos.secret_key",
        "tos.endpoint",
        "tos.region",
        "tos.bucket",
        "llm.api_key",
        "llm.model",
    ])
    assert_asr_auth_config(config)
