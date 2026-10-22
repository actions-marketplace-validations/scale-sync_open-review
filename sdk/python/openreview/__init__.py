"""Typed subprocess client for the open-review contract v2 CLI."""

from __future__ import annotations

import json
import os
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Literal, TypedDict

CONTRACT_VERSION = 2


class CoverageEntry(TypedDict):
    path: str
    status: str
    state: str
    patchBytes: int
    sentBytes: int


class Finding(TypedDict):
    id: str
    category: Literal["bug", "flag"]
    severity: Literal["severe", "non-severe", "investigate", "informational"]
    file: str
    startLine: int
    endLine: int
    title: str
    explanation: str
    source: str


class Result(TypedDict):
    contractVersion: int
    status: Literal["complete", "partial", "empty", "skipped", "failed"]
    coverage: list[CoverageEntry]
    findings: list[Finding]
    usage: dict[str, int | bool]
    request: dict[str, Any]


class ReviewError(RuntimeError):
    """A CLI, process, or contract failure."""


@dataclass(frozen=True)
class ReviewOptions:
    repo: str | Path
    mode: Literal["workspace", "commit", "branch", "scan"] = "workspace"
    commit: str | None = None
    base: str | None = None
    head: str | None = None
    scan_path: str | None = None
    model: str | None = None
    provider: str | None = None
    high_recall: bool = False
    binary: str | Path | None = None
    timeout: float | None = None


@dataclass(frozen=True)
class ProviderConfigOptions:
    provider: str
    model: str
    api_key: str
    base_url: str | None = None
    binary: str | Path | None = None


def configure_provider(options: ProviderConfigOptions) -> None:
    """Replace the saved BYOK provider and key without putting the secret in argv."""
    args = [str(options.binary or os.getenv("OPEN_REVIEW_BIN") or "open-review"),
            "config", "set", "--provider", options.provider, "--model", options.model,
            "--api-key-stdin"]
    if options.base_url:
        args.extend(("--base-url", options.base_url))
    try:
        process = subprocess.run(args, input=options.api_key + "\n", capture_output=True, text=True, check=False)
    except OSError as error:
        raise ReviewError(str(error)) from error
    if process.returncode != 0:
        raise ReviewError(process.stderr.strip() or "open-review config failed")


def review(options: ReviewOptions) -> Result:
    """Run the Go CLI and return its typed contract result.

    Partial coverage is returned as a result; invalid output and failed runs raise.
    A timeout terminates the subprocess.
    """
    args = [str(options.binary or os.getenv("OPEN_REVIEW_BIN") or "open-review"),
            "review", "--repo", str(options.repo), "--output", "json", "--quiet"]
    if options.mode == "workspace":
        args.append("--workspace")
    elif options.mode == "commit":
        if not options.commit:
            raise ReviewError("commit mode requires commit")
        args.extend(("--commit", options.commit))
    elif options.mode == "branch":
        if not options.base or not options.head:
            raise ReviewError("branch mode requires base and head")
        args.extend(("--base", options.base, "--head", options.head))
    elif options.mode == "scan":
        if not options.scan_path:
            raise ReviewError("scan mode requires scan_path")
        args.extend(("--scan", options.scan_path))
    else:
        raise ReviewError(f"unsupported mode {options.mode}")
    if options.model:
        args.extend(("--model", options.model))
    if options.provider:
        args.extend(("--provider", options.provider))
    if options.high_recall:
        args.append("--high-recall")

    try:
        process = subprocess.run(args, capture_output=True, text=True,
                                 timeout=options.timeout, check=False)
    except (OSError, subprocess.TimeoutExpired) as error:
        raise ReviewError(str(error)) from error
    try:
        result: Result = json.loads(process.stdout)
    except json.JSONDecodeError as error:
        raise ReviewError(process.stderr.strip() or "open-review returned invalid JSON") from error
    if result.get("contractVersion") != CONTRACT_VERSION:
        raise ReviewError(f"unsupported contract version {result.get('contractVersion')}")
    if process.returncode not in (0, 2):
        raise ReviewError(str(result.get("error") or process.stderr.strip() or "open-review failed"))
    return result


def scan(repo: str | Path, path: str, **kwargs: Any) -> Result:
    return review(ReviewOptions(repo=repo, mode="scan", scan_path=path, **kwargs))
