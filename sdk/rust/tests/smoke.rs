use openreview_sdk::{review, Mode, Options, CONTRACT_VERSION};

#[test]
fn empty_scan_contract() {
    let Some(binary) = std::env::var_os("OPEN_REVIEW_BIN") else { return; };
    let mut options = Options::new(std::env::temp_dir(), Mode::Scan(".".into()));
    options.binary = binary.into();
    // A fresh directory keeps this fixture free of model calls.
    let repo = std::env::temp_dir().join(format!("open-review-rust-{}", std::process::id()));
    std::fs::create_dir(&repo).unwrap();
    options.repo = repo.clone();
    let result = review(&options).unwrap();
    std::fs::remove_dir(repo).unwrap();
    assert_eq!(result.contract_version, CONTRACT_VERSION);
    assert_eq!(result.status, "empty");
    assert!(result.findings.is_empty());
}
