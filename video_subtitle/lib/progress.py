import json
import sys
import time
import uuid
from pathlib import Path

SENSITIVE_PROGRESS_KEYS = {
    "api_key",
    "authorization",
    "messages",
    "presigned_url",
    "prompt",
    "signed_url",
    "token",
    "url",
}


class ProgressLogger:
    def __init__(self, work_dir, config=None, stderr=None, run_id=None):
        self.work_dir = Path(work_dir)
        self.config = config or {}
        self.stderr = stderr if stderr is not None else sys.stderr
        self.run_id = run_id or uuid.uuid4().hex[:12]
        self.started_at = time.monotonic()
        self.preview_chars = int(self.config.get("logging", {}).get("stderr_preview_chars", 20) or 0)
        self.write_jsonl = bool(self.config.get("logging", {}).get("progress_jsonl", True))
        self.progress_path = self.work_dir / "progress.jsonl"

    def _elapsed_ms(self):
        return int((time.monotonic() - self.started_at) * 1000)

    def _safe_record(self, data):
        record = {}
        for key, value in data.items():
            if key in SENSITIVE_PROGRESS_KEYS:
                continue
            if key == "preview":
                continue
            record[key] = value
        return record

    def _preview(self, value):
        value = str(value or "")
        if self.preview_chars <= 0:
            return ""
        return value[:self.preview_chars]

    def event(self, stage, event, status="info", **fields):
        record = {
            "run_id": self.run_id,
            "elapsed_ms": self._elapsed_ms(),
            "stage": stage,
            "event": event,
            "status": status,
        }
        record.update(self._safe_record(fields))
        if self.write_jsonl:
            self.work_dir.mkdir(parents=True, exist_ok=True)
            with self.progress_path.open("a", encoding="utf-8") as f:
                json.dump(record, f, ensure_ascii=False, sort_keys=True)
                f.write("\n")
        parts = ["[%s]" % stage, event, status]
        for key in ("index", "total", "id_range", "time_range", "count", "elapsed_ms", "cache"):
            if key in record:
                parts.append("%s=%s" % (key, record[key]))
        if "error" in record:
            parts.append("error=%s" % record["error"])
        preview = self._preview(fields.get("preview"))
        if preview:
            parts.append("preview=%s" % preview)
        print(" ".join(parts), file=self.stderr)


class NullProgressLogger:
    run_id = ""

    def event(self, *args, **kwargs):
        return None
