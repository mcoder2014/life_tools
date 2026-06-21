import sys
import xml.etree.ElementTree as ET
from pathlib import Path

import requests


class LocalNfoContext(object):
    def __init__(self, tmdb_id="", summary_text=""):
        self.tmdb_id = tmdb_id
        self.summary_text = summary_text


def read_xml(path):
    text = Path(path).read_text(encoding="utf-8-sig")
    return ET.fromstring(text)


def first_text(root, names):
    for name in names:
        node = root.find(name)
        if node is not None and node.text:
            return node.text.strip()
    return ""


def parse_nfo(path):
    try:
        root = read_xml(path)
    except (OSError, ET.ParseError):
        return LocalNfoContext()

    parts = []
    tmdb_id = first_text(root, ["tmdbid", "./uniqueid[@type='tmdb']"])
    title = first_text(root, ["title", "originaltitle"])
    original_title = first_text(root, ["originaltitle"])
    plot = first_text(root, ["plot", "outline"])

    if title:
        parts.append("Title: " + title)
    if original_title and original_title != title:
        parts.append("Original title: " + original_title)
    if plot:
        parts.append("Plot: " + plot)

    actors = []
    for actor in root.findall("actor")[:20]:
        name = first_text(actor, ["name"])
        role = first_text(actor, ["role"])
        if name and role:
            actors.append("%s as %s" % (name, role))
        elif name:
            actors.append(name)
    if actors:
        parts.append("Cast: " + "; ".join(actors))

    return LocalNfoContext(tmdb_id=tmdb_id, summary_text="\n".join(parts))


def find_local_nfo_context(video_path):
    video_path = Path(video_path)
    candidates = [
        video_path.parent.parent / "tvshow.nfo",
        video_path.parent / "season.nfo",
        video_path.with_suffix(".nfo"),
    ]
    seen = set()
    contexts = []
    tmdb_id = ""
    for candidate in candidates:
        if candidate in seen or not candidate.exists():
            continue
        seen.add(candidate)
        context = parse_nfo(candidate)
        if context.tmdb_id and not tmdb_id:
            tmdb_id = context.tmdb_id
        if context.summary_text:
            contexts.append(context.summary_text)
    return LocalNfoContext(tmdb_id=tmdb_id, summary_text="\n\n".join(contexts))


def tmdb_headers_and_params(config):
    tmdb = config.get("tmdb", {})
    headers = {"Accept": "application/json"}
    params = {"language": tmdb.get("language", "zh-CN")}
    if tmdb.get("bearer_token"):
        headers["Authorization"] = "Bearer " + tmdb["bearer_token"]
    elif tmdb.get("api_key"):
        params["api_key"] = tmdb["api_key"]
    else:
        return None, None
    return headers, params


def fetch_tmdb_tv_details(tmdb_id, config):
    headers, params = tmdb_headers_and_params(config)
    if not headers:
        return ""
    url = "https://api.themoviedb.org/3/tv/%s" % tmdb_id
    resp = requests.get(url, headers=headers, params=params, timeout=30)
    resp.raise_for_status()
    data = resp.json()
    parts = []
    for key in ("name", "original_name", "overview"):
        if data.get(key):
            parts.append("%s: %s" % (key, data[key]))
    genres = [g.get("name") for g in data.get("genres", []) if g.get("name")]
    if genres:
        parts.append("genres: " + ", ".join(genres))
    return "\n".join(parts)


def search_tmdb_tv(query, config):
    headers, params = tmdb_headers_and_params(config)
    if not headers:
        return []
    params = dict(params)
    params["query"] = query
    resp = requests.get("https://api.themoviedb.org/3/search/tv", headers=headers, params=params, timeout=30)
    resp.raise_for_status()
    return resp.json().get("results", [])[:10]


def safe_fetch_tmdb_tv_details(tmdb_id, config):
    try:
        return fetch_tmdb_tv_details(tmdb_id, config)
    except requests.RequestException as e:
        print("warning: TMDB detail fetch failed: %s" % e, file=sys.stderr)
        return ""


def safe_search_tmdb_tv(query, config):
    try:
        return search_tmdb_tv(query, config)
    except requests.RequestException as e:
        print("warning: TMDB search failed: %s" % e, file=sys.stderr)
        return []


def choose_tmdb_candidate(candidates):
    if not candidates:
        return None
    print("TMDB candidates:")
    for idx, item in enumerate(candidates, start=1):
        name = item.get("name") or item.get("original_name") or ""
        date = item.get("first_air_date") or ""
        overview = (item.get("overview") or "").replace("\n", " ")[:100]
        print("%d. %s %s %s" % (idx, name, date, overview))
    while True:
        raw = input("Select TMDB candidate number, or empty to skip: ").strip()
        if not raw:
            return None
        try:
            index = int(raw)
        except ValueError:
            print("Please enter a number.")
            continue
        if 1 <= index <= len(candidates):
            return candidates[index - 1]
        print("Out of range.")


def build_background_context(video_path, config, allow_search_prompt):
    local = find_local_nfo_context(video_path)
    parts = []
    if local.summary_text:
        parts.append("Local NFO:\n" + local.summary_text)

    if local.tmdb_id:
        details = safe_fetch_tmdb_tv_details(local.tmdb_id, config)
        if details:
            parts.append("TMDB:\n" + details)
        return "\n\n".join(parts)

    if not allow_search_prompt:
        return "\n\n".join(parts)

    candidates = safe_search_tmdb_tv(Path(video_path).stem, config)
    selected = choose_tmdb_candidate(candidates) if allow_search_prompt else None
    if selected and selected.get("id"):
        details = safe_fetch_tmdb_tv_details(selected["id"], config)
        if details:
            parts.append("TMDB:\n" + details)
    return "\n\n".join(parts)
