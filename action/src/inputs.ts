import * as core from '@actions/core';

export function injectInputsToEnv(): void {
  const mappings: Array<[string, string]> = [
    ['github-token', 'GITHUB_TOKEN'],
    ['openai-api-key', 'OPENAI_API_KEY'],
    ['anthropic-api-key', 'ANTHROPIC_API_KEY'],
    ['gemini-api-key', 'GEMINI_API_KEY'],
    ['main-model', 'MAIN_MODEL'],
    ['provider', 'OPEN_REVIEW_PROVIDER'],
    ['base-url', 'OPEN_REVIEW_BASE_URL'],
    ['effort', 'OPEN_REVIEW_EFFORT'],
    ['prompt-tokens', 'OPEN_REVIEW_PROMPT_TOKENS'],
    ['output-tokens', 'OPEN_REVIEW_OUTPUT_TOKENS'],
    ['total-tokens', 'OPEN_REVIEW_TOTAL_TOKENS'],
    ['request-seconds', 'OPEN_REVIEW_REQUEST_SECONDS'],
    ['group-seconds', 'OPEN_REVIEW_GROUP_SECONDS'],
    ['sub-model', 'SUB_MODEL'],
    ['max-files', 'MAX_FILES'],
    ['review-drafts', 'REVIEW_DRAFTS'],
  ];

  for (const [input, envVar] of mappings) {
    const value = core.getInput(input);
    if (value) process.env[envVar] = value;
  }
}
