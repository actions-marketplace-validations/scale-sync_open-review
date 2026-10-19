'use client';

import { useMemo, useState } from 'react';

type Metrics = {
  cases: number;
  groundTruth: number;
  findings: number;
  truePositive: number;
  falsePositive: number;
  locatedMatches: number;
  precision: number;
  recall: number;
  f1: number;
  linePrecision: number;
  noiseRate: number;
  meanDurationMs: number;
  meanTotalTokens: number;
};

export type BenchmarkReport = {
  datasetSha256: string;
  reviewer: string;
  version: string;
  provider: string;
  model: string;
  effort: string;
  overall: Metrics;
  byLanguage: Record<string, Metrics>;
};

type SortField =
  | 'f1'
  | 'precision'
  | 'recall'
  | 'linePrecision'
  | 'noiseRate'
  | 'meanDurationMs'
  | 'meanTotalTokens';

const columns: { key: SortField; label: string; direction: 'higher' | 'lower' }[] = [
  { key: 'f1', label: 'F1', direction: 'higher' },
  { key: 'precision', label: 'Precision', direction: 'higher' },
  { key: 'recall', label: 'Recall', direction: 'higher' },
  { key: 'linePrecision', label: 'Line precision', direction: 'higher' },
  { key: 'noiseRate', label: 'Noise', direction: 'lower' },
  { key: 'meanDurationMs', label: 'Avg time', direction: 'lower' },
  { key: 'meanTotalTokens', label: 'Avg tokens', direction: 'lower' },
];

function display(field: SortField, value: number): string {
  if (field === 'meanDurationMs') return `${(value / 1000).toFixed(1)}s`;
  if (field === 'meanTotalTokens') return Math.round(value).toLocaleString();
  return `${(value * 100).toFixed(1)}%`;
}

export default function BenchmarkTable({ reports }: { reports: BenchmarkReport[] }) {
  const [language, setLanguage] = useState('All languages');
  const [sortField, setSortField] = useState<SortField>('f1');
  const [descending, setDescending] = useState(true);
  const languages = [
    ...new Set(reports.flatMap((report) => Object.keys(report.byLanguage))),
  ].sort();
  const sorted = useMemo(
    () =>
      reports
        .filter((report) => language === 'All languages' || report.byLanguage[language])
        .map((report) => ({
          report,
          metrics: language === 'All languages' ? report.overall : report.byLanguage[language],
        }))
        .sort((a, b) => {
          const delta = a.metrics[sortField] - b.metrics[sortField];
          return (descending ? -delta : delta) || a.report.model.localeCompare(b.report.model);
        }),
    [reports, language, sortField, descending],
  );

  function sortBy(field: SortField) {
    if (field === sortField) setDescending((value) => !value);
    else {
      setSortField(field);
      setDescending(columns.find((column) => column.key === field)?.direction === 'higher');
    }
  }

  return (
    <div className="benchmark-leaderboard">
      <div className="benchmark-table-top">
        <span>
          {reports.length ? `${reports.length} verified runs` : 'Results awaiting verified runs'}
        </span>
        <label>
          Language{' '}
          <select
            value={language}
            onChange={(event) => setLanguage(event.target.value)}
            disabled={!languages.length}
          >
            <option>All languages</option>
            {languages.map((item) => (
              <option key={item}>{item}</option>
            ))}
          </select>
        </label>
      </div>
      <div className="table-wrap">
        <table className="data-table benchmark-table">
          <thead>
            <tr>
              <th>Rank</th>
              <th>Model / reviewer</th>
              {columns.map((column) => (
                <th
                  aria-sort={
                    sortField === column.key ? (descending ? 'descending' : 'ascending') : 'none'
                  }
                  key={column.key}
                >
                  <button type="button" onClick={() => sortBy(column.key)}>
                    {column.label}{' '}
                    <span aria-hidden="true">
                      {sortField === column.key ? (descending ? '↓' : '↑') : '↕'}
                    </span>
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.length ? (
              sorted.map(({ report, metrics }, index) => (
                <tr
                  key={`${report.reviewer}:${report.version}:${report.provider}:${report.model}:${report.effort}`}
                >
                  <th>{index + 1}</th>
                  <td>
                    <strong>{report.model}</strong>
                    <small>
                      {report.provider} · {report.reviewer} {report.version} · {report.effort}
                    </small>
                  </td>
                  {columns.map((column) => (
                    <td key={column.key}>{display(column.key, metrics[column.key])}</td>
                  ))}
                </tr>
              ))
            ) : (
              <tr>
                <td className="benchmark-empty" colSpan={9}>
                  No scored runs yet. The evaluation pipeline is ready; public numbers need complete
                  reviews and checked judgments.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
