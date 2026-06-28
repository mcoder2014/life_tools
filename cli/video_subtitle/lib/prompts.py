from pathlib import Path
from string import Template

PROMPT_DIR = Path(__file__).resolve().parent.parent / "prompts"

DEFAULT_TEMPLATES = {
    "translation": PROMPT_DIR / "translation.txt",
    "split": PROMPT_DIR / "split.txt",
    "consistency": PROMPT_DIR / "consistency.txt",
}


def render_prompt_template(path, values):
    text = Path(path).read_text(encoding="utf-8")
    return Template(text).substitute(values)


def configured_template(config, key):
    configured = (config.get("prompts") or {}).get(key + "_template")
    if configured:
        return Path(configured)
    return DEFAULT_TEMPLATES[key]
