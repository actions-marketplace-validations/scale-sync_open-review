//! Typed subprocess client for the open-review contract v2 CLI.

use serde::Deserialize;
use std::io::{self, Read, Write};
use std::path::PathBuf;
use std::process::{Command, Stdio};
use std::time::{Duration, Instant};

pub const CONTRACT_VERSION: u32 = 2;

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CoverageEntry {
    pub path: String,
    pub status: String,
    pub state: String,
    pub reason: Option<String>,
    pub patch_bytes: usize,
    pub sent_bytes: usize,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Finding {
    pub id: String,
    pub category: String,
    pub severity: String,
    pub file: String,
    pub start_line: u32,
    pub end_line: u32,
    pub title: String,
    pub explanation: String,
    pub source: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Usage {
    pub prompt_tokens: u64,
    pub output_tokens: u64,
    pub total_tokens: u64,
    pub model_calls: u64,
    pub estimated: bool,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Result {
    pub contract_version: u32,
    pub status: String,
    pub coverage: Vec<CoverageEntry>,
    pub findings: Vec<Finding>,
    pub usage: Usage,
    pub error: Option<String>,
}

#[derive(Debug, Clone)]
pub enum Mode {
    Workspace,
    Commit(String),
    Branch { base: String, head: String },
    Scan(String),
}

#[derive(Debug, Clone)]
pub struct Options {
    pub repo: PathBuf,
    pub mode: Mode,
    pub binary: PathBuf,
    pub model: Option<String>,
    pub provider: Option<String>,
    pub high_recall: bool,
    pub timeout: Option<Duration>,
}

pub struct ProviderConfigOptions {
    pub provider: String,
    pub model: String,
    pub api_key: String,
    pub base_url: Option<String>,
    pub binary: PathBuf,
}

pub fn configure_provider(options: &ProviderConfigOptions) -> std::result::Result<(), ReviewError> {
    let mut command = Command::new(&options.binary);
    command.args(["config", "set", "--provider", &options.provider, "--model", &options.model, "--api-key-stdin"]);
    if let Some(base_url) = &options.base_url { command.args(["--base-url", base_url]); }
    let mut child = command.stdin(Stdio::piped()).stdout(Stdio::null()).stderr(Stdio::piped()).spawn().map_err(ReviewError::Process)?;
    child.stdin.take().expect("piped stdin").write_all(format!("{}\n", options.api_key).as_bytes()).map_err(ReviewError::Process)?;
    let output = child.wait_with_output().map_err(ReviewError::Process)?;
    if !output.status.success() {
        return Err(ReviewError::Failed(String::from_utf8_lossy(&output.stderr).trim().to_owned()));
    }
    Ok(())
}

impl Options {
    pub fn new(repo: impl Into<PathBuf>, mode: Mode) -> Self {
        Self {
            repo: repo.into(),
            mode,
            binary: std::env::var_os("OPEN_REVIEW_BIN")
                .map(PathBuf::from)
                .unwrap_or_else(|| PathBuf::from("open-review")),
            model: None,
            provider: None,
            high_recall: false,
            timeout: None,
        }
    }
}

#[derive(Debug)]
pub enum ReviewError {
    Process(io::Error),
    InvalidJson(serde_json::Error),
    Contract(u32),
    Failed(String),
    Timeout,
}

impl std::fmt::Display for ReviewError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Process(error) => write!(f, "open-review process: {error}"),
            Self::InvalidJson(error) => write!(f, "open-review JSON: {error}"),
            Self::Contract(version) => write!(f, "unsupported contract version {version}"),
            Self::Failed(message) => write!(f, "open-review failed: {message}"),
            Self::Timeout => write!(f, "open-review timed out"),
        }
    }
}

impl std::error::Error for ReviewError {}

pub fn review(options: &Options) -> std::result::Result<Result, ReviewError> {
    let mut command = Command::new(&options.binary);
    command.arg("review").arg("--repo").arg(&options.repo)
        .args(["--output", "json", "--quiet"])
        .stdin(Stdio::null()).stdout(Stdio::piped()).stderr(Stdio::piped());
    match &options.mode {
        Mode::Workspace => { command.arg("--workspace"); }
        Mode::Commit(commit) => { command.args(["--commit", commit]); }
        Mode::Branch { base, head } => { command.args(["--base", base, "--head", head]); }
        Mode::Scan(path) => { command.args(["--scan", path]); }
    }
    if let Some(model) = &options.model { command.args(["--model", model]); }
    if let Some(provider) = &options.provider { command.args(["--provider", provider]); }
    if options.high_recall { command.arg("--high-recall"); }

    let mut child = command.spawn().map_err(ReviewError::Process)?;
    let mut stdout = child.stdout.take().expect("piped stdout");
    let mut stderr = child.stderr.take().expect("piped stderr");
    let (status, stdout, stderr) = std::thread::scope(|scope| {
        let out = scope.spawn(move || {
            let mut bytes = Vec::new();
            stdout.read_to_end(&mut bytes).map(|_| bytes)
        });
        let err = scope.spawn(move || {
            let mut bytes = Vec::new();
            stderr.read_to_end(&mut bytes).map(|_| bytes)
        });
        let status = if let Some(timeout) = options.timeout {
            let deadline = Instant::now() + timeout;
            loop {
                if let Some(status) = child.try_wait().map_err(ReviewError::Process)? { break Ok(status); }
                if Instant::now() >= deadline {
                    let _ = child.kill();
                    let _ = child.wait();
                    break Err(ReviewError::Timeout);
                }
                std::thread::sleep(Duration::from_millis(10));
            }
        } else {
            child.wait().map_err(ReviewError::Process)
        };
        let stdout = out.join().expect("stdout reader").map_err(ReviewError::Process)?;
        let stderr = err.join().expect("stderr reader").map_err(ReviewError::Process)?;
        Ok::<_, ReviewError>((status?, stdout, stderr))
    })?;
    let result: Result = serde_json::from_slice(&stdout).map_err(ReviewError::InvalidJson)?;
    if result.contract_version != CONTRACT_VERSION {
        return Err(ReviewError::Contract(result.contract_version));
    }
    if !status.success() && status.code() != Some(2) {
        return Err(ReviewError::Failed(result.error.unwrap_or_else(||
            String::from_utf8_lossy(&stderr).trim().to_owned())));
    }
    Ok(result)
}

pub fn scan(repo: impl Into<PathBuf>, path: impl Into<String>) -> std::result::Result<Result, ReviewError> {
    review(&Options::new(repo, Mode::Scan(path.into())))
}
