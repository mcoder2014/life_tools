import uuid
from pathlib import Path

from .config import require_non_empty


def tos_object_key(video_path, audio_path, config):
    prefix = str(config["tos"].get("prefix", "video-subtitle/audio")).strip("/")
    return "%s/%s/%s" % (prefix, uuid.uuid4().hex, Path(audio_path).name)


def upload_audio_to_tos(audio_path, object_key, config):
    require_non_empty(config, [
        "tos.access_key",
        "tos.secret_key",
        "tos.endpoint",
        "tos.region",
        "tos.bucket",
    ])
    try:
        import tos
        from tos.enum import HttpMethodType
    except ImportError as e:
        raise RuntimeError("missing Python dependency: tos") from e

    tos_config = config["tos"]
    client = tos.TosClientV2(
        tos_config["access_key"],
        tos_config["secret_key"],
        tos_config["endpoint"],
        tos_config["region"],
    )
    client.put_object_from_file(tos_config["bucket"], object_key, str(audio_path))
    signed = client.pre_signed_url(
        HttpMethodType.Http_Method_Get,
        tos_config["bucket"],
        object_key,
        expires=int(tos_config.get("presigned_url_ttl_seconds", 86400)),
    )
    return signed.signed_url
