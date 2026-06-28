import contextlib
import importlib.util
import io
import json
import os
import subprocess
import tempfile
import threading
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("video_subtitle.py")


def load_module():
    spec = importlib.util.spec_from_file_location("video_subtitle", MODULE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class VideoSubtitleTest(unittest.TestCase):
    def test_legacy_entrypoint_reexports_public_functions(self):
        module = load_module()

        self.assertTrue(callable(module.run))
        self.assertTrue(callable(module.extract_utterances))
        self.assertTrue(callable(module.build_srt))
        self.assertTrue(callable(module.translate_utterances))

    def test_legacy_entrypoint_runs_as_script(self):
        result = subprocess.run(
            [os.environ.get("PYTHON", "python3"), str(MODULE_PATH), "--help"],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

        self.assertEqual("", result.stderr)
        self.assertEqual(0, result.returncode)
        self.assertIn("Generate Chinese SRT", result.stdout)

    def test_legacy_entrypoint_help_includes_force_split(self):
        result = subprocess.run(
            [os.environ.get("PYTHON", "python3"), str(MODULE_PATH), "--help"],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

        self.assertEqual(0, result.returncode)
        self.assertIn("--force-split", result.stdout)

    def test_choose_output_path_uses_zh_cn_suffix_and_counts_to_100(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            video = Path(tmp) / "Episode 01.mp4"
            video.touch()
            expected = Path(tmp) / "Episode 01.zh-CN.srt"

            self.assertEqual(expected, module.choose_output_path(video, None))

            expected.touch()
            self.assertEqual(Path(tmp) / "Episode 01.zh-CN_1.srt", module.choose_output_path(video, None))

            for i in range(1, 101):
                (Path(tmp) / f"Episode 01.zh-CN_{i}.srt").touch()

            with self.assertRaisesRegex(RuntimeError, "100"):
                module.choose_output_path(video, None)

    def test_config_environment_values_override_json_values(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            config_path = Path(tmp) / "video_subtitle.json"
            config_path.write_text(
                """{
                  "asr": {"api_key": "json-asr", "resource_id": "json-resource"},
                  "llm": {"api_key": "json-llm", "model": "json-model"},
                  "tos": {"access_key": "json-ak", "secret_key": "json-sk", "endpoint": "json-endpoint", "region": "json-region", "bucket": "json-bucket"}
                }""",
                encoding="utf-8",
            )
            env = {
                "VIDEO_SUBTITLE_ASR_API_KEY": "env-asr",
                "VIDEO_SUBTITLE_LLM_MODEL": "env-model",
                "VIDEO_SUBTITLE_TOS_BUCKET": "env-bucket",
            }

            config = module.load_config(config_path, env)

            self.assertEqual("env-asr", config["asr"]["api_key"])
            self.assertEqual("json-resource", config["asr"]["resource_id"])
            self.assertEqual("env-model", config["llm"]["model"])
            self.assertEqual("env-bucket", config["tos"]["bucket"])

    def test_extract_utterances_rejects_missing_utterance_times(self):
        module = load_module()
        payload = {
            "result": {
                "utterances": [
                    {"start_time": 0, "end_time": 1200, "text": "こんにちは"},
                    {"start_time": 1500, "text": "エミリア"},
                ]
            }
        }

        with self.assertRaisesRegex(ValueError, "end_time"):
            module.extract_utterances(payload)

    def test_extract_utterances_preserves_word_timings(self):
        module = load_module()
        payload = {
            "result": {
                "utterances": [
                    {
                        "start_time": 1000,
                        "end_time": 2500,
                        "text": "お兄ちゃん",
                        "words": [
                            {"start_time": 1000, "end_time": 1300, "text": "お"},
                            {"start_time": 1300, "end_time": 1800, "text": "兄"},
                            {"start_time": 1800, "end_time": 2500, "text": "ちゃん"},
                        ],
                    }
                ]
            }
        }

        self.assertEqual(
            [
                {
                    "id": 1,
                    "start_ms": 1000,
                    "end_ms": 2500,
                    "text": "お兄ちゃん",
                    "words": [
                        {"start_ms": 1000, "end_ms": 1300, "text": "お"},
                        {"start_ms": 1300, "end_ms": 1800, "text": "兄"},
                        {"start_ms": 1800, "end_ms": 2500, "text": "ちゃん"},
                    ],
                }
            ],
            module.extract_utterances(payload),
        )

    def test_extract_utterances_filters_invalid_blank_words(self):
        module = load_module()
        payload = {
            "result": {
                "utterances": [
                    {
                        "start_time": 1000,
                        "end_time": 5000,
                        "text": "だけどその前に どこだここ。",
                        "words": [
                            {"start_time": 1000, "end_time": 1100, "text": "だ"},
                            {"start_time": -1, "end_time": -1, "text": " "},
                            {"start_time": 2000, "end_time": 2100, "text": "ど"},
                            {"start_time": 2200, "end_time": 2200, "text": ""},
                            {"start_time": 2300, "end_time": 2200, "text": "bad"},
                        ],
                    }
                ]
            }
        }

        self.assertEqual(
            [
                {"start_ms": 1000, "end_ms": 1100, "text": "だ"},
                {"start_ms": 2000, "end_ms": 2100, "text": "ど"},
            ],
            module.extract_utterances(payload)[0]["words"],
        )

    def test_build_srt_uses_translated_text_and_asr_timing(self):
        module = load_module()
        utterances = [
            {"id": 1, "start_ms": 0, "end_ms": 1250, "text": "こんにちは"},
            {"id": 2, "start_ms": 1500, "end_ms": 3000, "text": "エミリア"},
        ]
        translations = {1: "你好。", 2: "爱蜜莉雅。"}

        self.assertEqual(
            "1\n00:00:00,000 --> 00:00:01,250\n你好。\n\n"
            "2\n00:00:01,500 --> 00:00:03,000\n爱蜜莉雅。\n",
            module.build_srt(utterances, translations),
        )

    def test_render_prompt_template_uses_standard_template_variables(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            template = Path(tmp) / "translation.txt"
            template.write_text("背景=$background\n数据=$items_json", encoding="utf-8")

            self.assertEqual(
                "背景=世界观\n数据=[1, 2]",
                module.render_prompt_template(template, {"background": "世界观", "items_json": "[1, 2]"}),
            )

    def test_build_subtitle_units_from_word_ranges_uses_word_timing(self):
        module = load_module()
        utterance = {
            "id": 1,
            "start_ms": 1000,
            "end_ms": 5000,
            "text": "abcdef",
            "words": [
                {"start_ms": 1000, "end_ms": 1500, "text": "a"},
                {"start_ms": 1500, "end_ms": 2000, "text": "b"},
                {"start_ms": 2000, "end_ms": 2500, "text": "c"},
                {"start_ms": 2500, "end_ms": 3500, "text": "d"},
                {"start_ms": 3500, "end_ms": 4200, "text": "e"},
                {"start_ms": 4200, "end_ms": 5000, "text": "f"},
            ],
        }
        ranges = [
            {"source_id": 1, "word_start": 0, "word_end": 2, "text": "abc"},
            {"source_id": 1, "word_start": 3, "word_end": 5, "text": "def"},
        ]

        self.assertEqual(
            [
                {"id": 1, "source_id": 1, "start_ms": 1000, "end_ms": 2500, "text": "abc"},
                {"id": 2, "source_id": 1, "start_ms": 2500, "end_ms": 5000, "text": "def"},
            ],
            module.build_subtitle_units_from_word_ranges([utterance], ranges),
        )

    def test_build_subtitle_units_rejects_overlapping_word_ranges(self):
        module = load_module()
        utterance = {
            "id": 1,
            "start_ms": 1000,
            "end_ms": 3000,
            "text": "abc",
            "words": [
                {"start_ms": 1000, "end_ms": 1500, "text": "a"},
                {"start_ms": 1500, "end_ms": 2000, "text": "b"},
                {"start_ms": 2000, "end_ms": 3000, "text": "c"},
            ],
        }
        ranges = [
            {"source_id": 1, "word_start": 0, "word_end": 1, "text": "ab"},
            {"source_id": 1, "word_start": 1, "word_end": 2, "text": "bc"},
        ]

        with self.assertRaisesRegex(ValueError, "overlap"):
            module.build_subtitle_units_from_word_ranges([utterance], ranges)

    def test_build_subtitle_units_rejects_missing_word_ranges(self):
        module = load_module()
        utterance = {
            "id": 1,
            "start_ms": 1000,
            "end_ms": 3000,
            "text": "abc",
            "words": [
                {"start_ms": 1000, "end_ms": 1500, "text": "a"},
                {"start_ms": 1500, "end_ms": 2000, "text": "b"},
                {"start_ms": 2000, "end_ms": 3000, "text": "c"},
            ],
        }

        with self.assertRaisesRegex(ValueError, "gap"):
            module.build_subtitle_units_from_word_ranges(
                [utterance],
                [{"source_id": 1, "word_start": 1, "word_end": 2, "text": "bc"}],
            )
        with self.assertRaisesRegex(ValueError, "cover all words"):
            module.build_subtitle_units_from_word_ranges(
                [utterance],
                [{"source_id": 1, "word_start": 0, "word_end": 1, "text": "ab"}],
            )

    def test_build_split_chunks_uses_time_word_and_utterance_limits(self):
        module = load_module()

        def utterance(item_id, start_ms, end_ms, word_count):
            return {
                "id": item_id,
                "start_ms": start_ms,
                "end_ms": end_ms,
                "text": "u%d" % item_id,
                "words": [
                    {"start_ms": start_ms + i, "end_ms": start_ms + i + 1, "text": str(i)}
                    for i in range(word_count)
                ],
            }

        config = module.default_config()
        config["subtitle_split"].update({
            "max_window_seconds": 60,
            "max_words_per_chunk": 120,
            "max_utterances_per_chunk": 8,
        })

        chunks = module.build_split_chunks([
            utterance(1, 0, 10000, 40),
            utterance(2, 10000, 20000, 40),
            utterance(3, 20000, 30000, 50),
            utterance(4, 95000, 96000, 10),
        ], config)

        self.assertEqual([[1, 2], [3], [4]], [chunk["source_ids"] for chunk in chunks])
        self.assertEqual([80, 50, 10], [chunk["word_count"] for chunk in chunks])

        config["subtitle_split"].update({
            "max_window_seconds": 3600,
            "max_words_per_chunk": 1000,
            "max_utterances_per_chunk": 2,
        })
        chunks = module.build_split_chunks([
            utterance(1, 0, 1000, 1),
            utterance(2, 1000, 2000, 1),
            utterance(3, 2000, 3000, 1),
        ], config)

        self.assertEqual([[1, 2], [3]], [chunk["source_ids"] for chunk in chunks])

    def test_split_utterances_uses_chunk_cache(self):
        module = load_module()
        utterances = [
            {
                "id": 1,
                "start_ms": 0,
                "end_ms": 2000,
                "text": "ab",
                "words": [
                    {"start_ms": 0, "end_ms": 1000, "text": "a"},
                    {"start_ms": 1000, "end_ms": 2000, "text": "b"},
                ],
            }
        ]
        config = module.default_config()
        calls = []

        def fake_create_client(config):
            return object()

        def fake_split(client, llm, batch, config, raw_response_dir=None, batch_label="split"):
            calls.append(batch_label)
            return [{"source_id": 1, "word_start": 0, "word_end": 1, "text": "ab"}]

        globals_ = module.split_utterances_with_llm.__globals__
        old_create = globals_["create_openai_client"]
        old_split = globals_["split_batch_with_llm"]
        try:
            globals_["create_openai_client"] = fake_create_client
            globals_["split_batch_with_llm"] = fake_split
            with tempfile.TemporaryDirectory() as tmp:
                first = module.split_utterances_with_llm(utterances, config, Path(tmp))
                self.assertEqual(1, len(calls))

                def fail_split(*args, **kwargs):
                    raise AssertionError("cached split chunk should be used")

                globals_["split_batch_with_llm"] = fail_split
                second = module.split_utterances_with_llm(utterances, config, Path(tmp))
        finally:
            globals_["create_openai_client"] = old_create
            globals_["split_batch_with_llm"] = old_split

        self.assertEqual(first, second)

    def test_split_utterances_falls_back_failed_chunk_only(self):
        module = load_module()
        utterances = [
            {
                "id": 1,
                "start_ms": 0,
                "end_ms": 2000,
                "text": "ab",
                "words": [
                    {"start_ms": 0, "end_ms": 1000, "text": "a"},
                    {"start_ms": 1000, "end_ms": 2000, "text": "b"},
                ],
            },
            {
                "id": 2,
                "start_ms": 3000,
                "end_ms": 6000,
                "text": "cde",
                "words": [
                    {"start_ms": 3000, "end_ms": 4000, "text": "c"},
                    {"start_ms": 4000, "end_ms": 5000, "text": "d"},
                    {"start_ms": 5000, "end_ms": 6000, "text": "e"},
                ],
            },
        ]
        config = module.default_config()
        config["subtitle_split"]["max_utterances_per_chunk"] = 1
        config["llm"]["max_batch_retries"] = 1
        config["llm"]["retry_base_delay_seconds"] = 0

        def fake_create_client(config):
            return object()

        def fake_split(client, llm, batch, config, raw_response_dir=None, batch_label="split"):
            if batch[0]["id"] == 1:
                raise RuntimeError("llm timeout")
            return [
                {"source_id": 2, "word_start": 0, "word_end": 1, "text": "cd"},
                {"source_id": 2, "word_start": 2, "word_end": 2, "text": "e"},
            ]

        globals_ = module.split_utterances_with_llm.__globals__
        old_create = globals_["create_openai_client"]
        old_split = globals_["split_batch_with_llm"]
        try:
            globals_["create_openai_client"] = fake_create_client
            globals_["split_batch_with_llm"] = fake_split
            with tempfile.TemporaryDirectory() as tmp:
                units = module.split_utterances_with_llm(utterances, config, Path(tmp))
        finally:
            globals_["create_openai_client"] = old_create
            globals_["split_batch_with_llm"] = old_split

        self.assertEqual(
            [
                {"id": 1, "source_id": 1, "start_ms": 0, "end_ms": 2000, "text": "ab"},
                {"id": 2, "source_id": 2, "start_ms": 3000, "end_ms": 5000, "text": "cd"},
                {"id": 3, "source_id": 2, "start_ms": 5000, "end_ms": 6000, "text": "e"},
            ],
            units,
        )

    def test_progress_logger_writes_jsonl_without_sensitive_payloads(self):
        module = load_module()
        stderr = io.StringIO()
        config = module.default_config()
        with tempfile.TemporaryDirectory() as tmp:
            logger = module.ProgressLogger(Path(tmp), config, stderr=stderr, run_id="run-1")
            logger.event(
                "llm_split",
                "chunk_start",
                status="start",
                id_range="1-3",
                prompt="do not write prompt",
                presigned_url="https://example.invalid/secret",
                preview="abcdefghijklmnopqrstuvwxyz",
            )
            progress_text = (Path(tmp) / "progress.jsonl").read_text(encoding="utf-8")
            record = json.loads(progress_text.strip())

        self.assertEqual("run-1", record["run_id"])
        self.assertEqual("llm_split", record["stage"])
        self.assertNotIn("prompt", record)
        self.assertNotIn("presigned_url", record)
        self.assertNotIn("abcdefghijklmnopqrstuvwxyz", progress_text)
        self.assertIn("abcdefghijklmnopqrst", stderr.getvalue())
        self.assertNotIn("uvwxyz", stderr.getvalue())

    def test_generate_subtitle_force_translate_does_not_force_split(self):
        module = load_module()
        calls = []
        temp_dirs = []

        def fake_work_dir_for(video_path):
            tmp = tempfile.mkdtemp()
            temp_dirs.append(tmp)
            return Path(tmp)

        def fake_load_or_run_asr(video_path, work_dir, source_language, config, force_asr, logger=None):
            return [{"id": 1, "start_ms": 0, "end_ms": 1000, "text": "a"}]

        def fake_load_or_build(video_path, utterances, work_dir, config, force_asr=False, force_split=False, logger=None):
            calls.append({"force_asr": force_asr, "force_split": force_split})
            return [{"id": 1, "start_ms": 0, "end_ms": 1000, "text": "a"}]

        def fake_load_or_translate(video_path, subtitle_units, work_dir, config, force_translate, allow_search_prompt, logger=None):
            return {1: "甲"}

        globals_ = module.generate_subtitle.__globals__
        old_work = globals_.get("work_dir_for")
        old_asr = globals_["load_or_run_asr"]
        old_units = globals_["load_or_build_subtitle_units"]
        old_translate = globals_["load_or_translate"]
        try:
            globals_["work_dir_for"] = fake_work_dir_for
            globals_["load_or_run_asr"] = fake_load_or_run_asr
            globals_["load_or_build_subtitle_units"] = fake_load_or_build
            globals_["load_or_translate"] = fake_load_or_translate
            with tempfile.TemporaryDirectory() as tmp:
                with contextlib.redirect_stderr(io.StringIO()):
                    module.generate_subtitle(
                        Path(tmp) / "video.mkv",
                        Path(tmp) / "video.zh-CN.srt",
                        "ja-JP",
                        module.default_config(),
                        force_asr=False,
                        force_translate=True,
                        allow_search_prompt=False,
                        force_split=False,
                    )
        finally:
            if old_work is None:
                globals_.pop("work_dir_for", None)
            else:
                globals_["work_dir_for"] = old_work
            globals_["load_or_run_asr"] = old_asr
            globals_["load_or_build_subtitle_units"] = old_units
            globals_["load_or_translate"] = old_translate

        self.assertEqual([{"force_asr": False, "force_split": False}], calls)

    def test_should_adopt_embedded_candidate_uses_configured_thresholds(self):
        module = load_module()
        config = module.default_config()
        good = {"time_coverage": 0.81, "aligned_ratio": 0.72, "llm_consistency": 0.83}
        bad = {"time_coverage": 0.81, "aligned_ratio": 0.69, "llm_consistency": 0.95}

        self.assertTrue(module.should_adopt_embedded_candidate(good, config))
        self.assertFalse(module.should_adopt_embedded_candidate(bad, config))

    def test_mechanical_score_aligns_by_time_overlap(self):
        module = load_module()
        asr = [
            {"id": 1, "start_ms": 0, "end_ms": 1000, "text": "a"},
            {"id": 2, "start_ms": 1500, "end_ms": 2500, "text": "b"},
        ]
        embedded = [
            {"id": 1, "start_ms": 0, "end_ms": 900, "text": "A"},
            {"id": 2, "start_ms": 1600, "end_ms": 2400, "text": "B"},
        ]

        score = module.mechanical_score(asr, embedded)

        self.assertAlmostEqual(0.85, score["time_coverage"])
        self.assertAlmostEqual(1.0, score["aligned_ratio"])
        self.assertEqual(2, len(score["aligned"]))

    def test_subtitle_units_signature_changes_when_text_changes(self):
        module = load_module()
        base = [{"id": 1, "start_ms": 0, "end_ms": 1000, "text": "a"}]
        changed = [{"id": 1, "start_ms": 0, "end_ms": 1000, "text": "b"}]

        self.assertNotEqual(
            module.subtitle_units_signature(base),
            module.subtitle_units_signature(changed),
        )

    def test_translation_cache_requires_matching_subtitle_units_signature(self):
        module = load_module()
        units = [{"id": 1, "start_ms": 0, "end_ms": 1000, "text": "a"}]
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            module.write_json(work / "translations.json", {"1": "甲"})

            self.assertIsNone(module.read_cached_translations(work, units))

            module.write_translation_cache(work, units, {1: "甲"})
            self.assertEqual({1: "甲"}, module.read_cached_translations(work, units))

    def test_parse_translation_response_requires_matching_ids(self):
        module = load_module()
        response = '[{"id": 1, "text": "你好。"}, {"id": 3, "text": "错位。"}]'

        with self.assertRaisesRegex(ValueError, "missing"):
            module.parse_translation_response(response, expected_ids=[1, 2])

    def test_parse_translation_response_accepts_markdown_json_fence(self):
        module = load_module()
        response = '```json\n[{"id": 1, "text": "你好。"}]\n```'

        self.assertEqual(
            {1: "你好。"},
            module.parse_translation_response(response, expected_ids=[1]),
        )

    def test_parse_translation_response_extracts_json_from_wrapped_text(self):
        module = load_module()
        response = '以下是结果：\n[{"id": 1, "text": "你好。"}]\n请查收。'

        self.assertEqual(
            {1: "你好。"},
            module.parse_translation_response(response, expected_ids=[1]),
        )

    def test_run_llm_jobs_retries_failed_job_with_annealing_limit(self):
        module = load_module()
        attempts = []

        def worker(job, attempt):
            attempts.append(attempt)
            if attempt < 3:
                raise RuntimeError("temporary")
            return "ok"

        results = module.run_llm_jobs(
            [{"label": "job-1"}],
            worker,
            module.default_config(),
            stage="unit",
            logger=module.NullProgressLogger(),
            sleep_func=lambda seconds: None,
        )

        self.assertEqual(["ok"], results)
        self.assertEqual([1, 2, 3], attempts)

    def test_run_llm_jobs_honors_configured_concurrency(self):
        module = load_module()
        config = module.default_config()
        config["llm"].update({"parallel_requests": 2, "max_batch_retries": 1})
        lock = threading.Lock()
        active = 0
        max_active = 0
        release = threading.Event()
        entered = threading.Event()

        def worker(job, attempt):
            nonlocal active, max_active
            with lock:
                active += 1
                max_active = max(max_active, active)
                if active == 2:
                    entered.set()
            entered.wait(2)
            release.wait(2)
            with lock:
                active -= 1
            return job["label"]

        thread = threading.Thread(
            target=lambda: module.run_llm_jobs(
                [{"label": "a"}, {"label": "b"}, {"label": "c"}],
                worker,
                config,
                stage="unit",
                logger=module.NullProgressLogger(),
                sleep_func=lambda seconds: None,
            )
        )
        thread.start()
        self.assertTrue(entered.wait(2))
        self.assertEqual(2, max_active)
        release.set()
        thread.join(2)
        self.assertFalse(thread.is_alive())

    def test_translate_batch_with_retry_splits_bad_batch(self):
        module = load_module()
        utterances = [
            {"id": 1, "text": "a"},
            {"id": 2, "text": "b"},
        ]
        calls = []

        def call_model(batch):
            calls.append([item["id"] for item in batch])
            if len(batch) > 1:
                return "not json"
            return '[{"id": %d, "text": "ok-%d"}]' % (batch[0]["id"], batch[0]["id"])

        self.assertEqual(
            {1: "ok-1", 2: "ok-2"},
            module.translate_batch_with_retry(call_model, utterances, max_attempts=1),
        )
        self.assertEqual([[1, 2], [1], [2]], calls)

    def test_build_response_format_prefers_strict_json_schema(self):
        module = load_module()
        response_format = module.build_response_format("json_schema")

        self.assertEqual("json_schema", response_format["type"])
        self.assertTrue(response_format["json_schema"]["strict"])
        self.assertIn("items", response_format["json_schema"]["schema"]["properties"])

    def test_find_nfo_context_prefers_tvshow_tmdb_id(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            season = Path(tmp) / "Season 3"
            season.mkdir()
            (Path(tmp) / "tvshow.nfo").write_text(
                """<?xml version="1.0" encoding="utf-8"?>
                <tvshow>
                  <title>Re:Zero</title>
                  <tmdbid>65942</tmdbid>
                  <plot>Plot from NFO.</plot>
                  <actor><name>Rie Takahashi</name><role>Emilia (voice)</role></actor>
                </tvshow>""",
                encoding="utf-8",
            )
            video = season / "ReZero S03E01.mp4"
            video.touch()

            context = module.find_local_nfo_context(video)

            self.assertEqual("65942", context.tmdb_id)
            self.assertIn("Re:Zero", context.summary_text)
            self.assertIn("Emilia", context.summary_text)

    def test_load_or_run_asr_refreshes_old_cached_utterances_without_words(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            module.write_json(work / "asr_result.json", {
                "result": {
                    "utterances": [
                        {
                            "start_time": 1000,
                            "end_time": 1800,
                            "text": "あい",
                            "words": [
                                {"start_time": 1000, "end_time": 1400, "text": "あ"},
                                {"start_time": 1400, "end_time": 1800, "text": "い"},
                            ],
                        }
                    ]
                }
            })
            module.write_json(work / "utterances.json", [
                {"id": 1, "start_ms": 1000, "end_ms": 1800, "text": "あい"}
            ])

            utterances = module.load_or_run_asr(Path(tmp) / "video.mkv", work, "ja-JP", module.default_config(), False)

            self.assertIn("words", utterances[0])
            self.assertEqual("あ", utterances[0]["words"][0]["text"])

    def test_load_or_run_asr_refreshes_cached_utterances_with_invalid_words(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            work = Path(tmp)
            module.write_json(work / "asr_result.json", {
                "result": {
                    "utterances": [
                        {
                            "start_time": 1000,
                            "end_time": 3000,
                            "text": "あ い",
                            "words": [
                                {"start_time": 1000, "end_time": 1400, "text": "あ"},
                                {"start_time": -1, "end_time": -1, "text": " "},
                                {"start_time": 2400, "end_time": 2800, "text": "い"},
                            ],
                        }
                    ]
                }
            })
            module.write_json(work / "utterances.json", [
                {
                    "id": 1,
                    "start_ms": 1000,
                    "end_ms": 3000,
                    "text": "あ い",
                    "words": [
                        {"start_ms": 1000, "end_ms": 1400, "text": "あ"},
                        {"start_ms": -1, "end_ms": -1, "text": " "},
                        {"start_ms": 2400, "end_ms": 2800, "text": "い"},
                    ],
                }
            ])

            utterances = module.load_or_run_asr(Path(tmp) / "video.mkv", work, "ja-JP", module.default_config(), False)

            self.assertEqual(
                [
                    {"start_ms": 1000, "end_ms": 1400, "text": "あ"},
                    {"start_ms": 2400, "end_ms": 2800, "text": "い"},
                ],
                utterances[0]["words"],
            )
            cached = module.read_json(work / "utterances.json")
            self.assertEqual(utterances, cached)

    def test_background_context_does_not_search_tmdb_when_prompt_is_disabled(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            video = Path(tmp) / "Unknown Show S01E01.mp4"
            video.touch()

            def fail_search(query, config):
                raise AssertionError("tmdb search should not be called")

            module.search_tmdb_tv = fail_search

            self.assertEqual("", module.build_background_context(video, module.default_config(), False))

    def test_background_context_keeps_local_nfo_when_tmdb_times_out(self):
        module = load_module()
        with tempfile.TemporaryDirectory() as tmp:
            season = Path(tmp) / "Season 3"
            season.mkdir()
            (Path(tmp) / "tvshow.nfo").write_text(
                """<?xml version="1.0" encoding="utf-8"?>
                <tvshow>
                  <title>Re:Zero</title>
                  <tmdbid>65942</tmdbid>
                  <plot>Plot from NFO.</plot>
                </tvshow>""",
                encoding="utf-8",
            )
            video = season / "ReZero S03E01.mp4"
            video.touch()

            def timeout(tmdb_id, config):
                raise module.requests.Timeout("tmdb timed out")

            module.fetch_tmdb_tv_details = timeout

            with contextlib.redirect_stderr(io.StringIO()):
                context = module.build_background_context(video, module.default_config(), False)

            self.assertIn("Local NFO", context)
            self.assertIn("Re:Zero", context)

    def test_validate_runtime_config_fails_before_expensive_work(self):
        module = load_module()
        config = module.default_config()

        with self.assertRaisesRegex(RuntimeError, "tos.access_key"):
            module.validate_runtime_config(config)


if __name__ == "__main__":
    unittest.main()
