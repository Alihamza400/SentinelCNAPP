'use client';

import { useState, useEffect } from 'react';
import { useAuth } from '@/components/auth-provider';
import { AlertTriangle, Search, Shield, Filter } from 'lucide-react';

interface Finding {
  id: string;
  source: string;
  type: string;
  severity: string;
  title: string;
  description: string;
  asset_id: string;
  status: string;
  detected_at: string;
  risk_score?: number;
}

interface FindingsResponse {
  findings: Finding[];
  total: number;
  page: number;
  page_size: number;
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-400',
  high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/20 dark:text-orange-400',
  medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400',
  low: 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400',
  info: 'bg-gray-100 text-gray-700 dark:bg-gray-900/20 dark:text-gray-400',
};

const SOURCE_ICONS: Record<string, string> = {
  checkov: '🏗️',
  trivy: '🐳',
  gitleaks: '🔑',
  falco: '🛡️',
  'k8s-custom': '☸️',
};

export default function FindingsPage() {
  const { isAuthenticated } = useAuth();
  const [findings, setFindings] = useState<Finding[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [severityFilter, setSeverityFilter] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 25;

  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchFindings = async () => {
      setLoading(true);
      setError('');

      const params = new URLSearchParams();
      if (severityFilter) params.set('severity', severityFilter);
      if (sourceFilter) params.set('source', sourceFilter);
      if (search) params.set('search', search);
      params.set('page', String(page));
      params.set('page_size', String(pageSize));

      try {
        const token = localStorage.getItem('sentinel_token');
        const res = await fetch(`http://localhost:8080/api/v1/findings?${params}`, {
          headers: { Authorization: `Bearer ${token}` },
        });

        if (res.ok) {
          const data: FindingsResponse = await res.json();
          setFindings(data.findings);
          setTotal(data.total);
        } else {
          // Findings API not ready yet (Phase 3), use mock data
          setFindings(getMockFindings());
          setTotal(getMockFindings().length);
        }
      } catch {
        // Use mock data when backend is not available
        setFindings(getMockFindings());
        setTotal(getMockFindings().length);
      } finally {
        setLoading(false);
      }
    };

    fetchFindings();
  }, [isAuthenticated, severityFilter, sourceFilter, search, page]);

  if (!isAuthenticated) {
    return (
      <div className="container py-6">
        <p className="text-muted-foreground">Please log in to view findings.</p>
      </div>
    );
  }

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="container py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Findings</h1>
          <p className="text-muted-foreground mt-1">{total} total findings</p>
        </div>
        <AlertTriangle className="h-8 w-8 text-muted-foreground" />
      </div>

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search findings..."
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            className="w-full rounded-md border bg-background pl-9 pr-3 py-2 text-sm"
          />
        </div>

        <select
          value={severityFilter}
          onChange={(e) => { setSeverityFilter(e.target.value); setPage(1); }}
          className="rounded-md border bg-background px-3 py-2 text-sm"
        >
          <option value="">All Severities</option>
          <option value="critical">Critical</option>
          <option value="high">High</option>
          <option value="medium">Medium</option>
          <option value="low">Low</option>
        </select>

        <select
          value={sourceFilter}
          onChange={(e) => { setSourceFilter(e.target.value); setPage(1); }}
          className="rounded-md border bg-background px-3 py-2 text-sm"
        >
          <option value="">All Sources</option>
          <option value="checkov">Checkov (IaC)</option>
          <option value="trivy">Trivy (Container)</option>
          <option value="gitleaks">Gitleaks (Secrets)</option>
          <option value="k8s-custom">K8s Custom</option>
        </select>
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">{error}</div>
      )}

      {loading ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <p className="text-muted-foreground">Loading findings...</p>
        </div>
      ) : findings.length === 0 ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <p className="text-muted-foreground">No findings yet. Run a scan to populate findings.</p>
        </div>
      ) : (
        <>
          <div className="rounded-lg border bg-card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="text-left p-3 font-medium w-8"></th>
                  <th className="text-left p-3 font-medium">Severity</th>
                  <th className="text-left p-3 font-medium">Title</th>
                  <th className="text-left p-3 font-medium">Source</th>
                  <th className="text-left p-3 font-medium">Type</th>
                  <th className="text-left p-3 font-medium">Asset</th>
                    <th className="text-left p-3 font-medium">Risk Score</th>
                  <th className="text-left p-3 font-medium">Status</th>
                  <th className="text-left p-3 font-medium">Detected</th>
                </tr>
              </thead>
              <tbody>
                {findings.map((finding) => (
                  <tr key={finding.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                    <td className="p-3 text-lg">{SOURCE_ICONS[finding.source] || '🔍'}</td>
                    <td className="p-3">
                      <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_COLORS[finding.severity] || SEVERITY_COLORS.info}`}>
                        {finding.severity}
                      </span>
                    </td>
                    <td className="p-3 font-medium max-w-md truncate" title={finding.title}>
                      {finding.title}
                    </td>
                    <td className="p-3 text-muted-foreground capitalize">{finding.source}</td>
                    <td className="p-3 text-muted-foreground">{finding.type}</td>
                    <td className="p-3 text-xs text-muted-foreground max-w-[150px] truncate" title={finding.asset_id}>
                      {finding.asset_id}
                    </td>
                    <td className="p-3">
                      <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                        finding.risk_score !== undefined && finding.risk_score >= 7
                          ? 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-400'
                          : finding.risk_score !== undefined && finding.risk_score >= 4
                          ? 'bg-orange-100 text-orange-700 dark:bg-orange-900/20 dark:text-orange-400'
                          : 'bg-gray-100 text-gray-700 dark:bg-gray-900/20 dark:text-gray-400'
                      }`}>
                        {finding.risk_score !== undefined ? finding.risk_score.toFixed(1) : '--'}
                      </span>
                    </td>
                    <td className="p-3">
                      <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                        finding.status === 'open'
                          ? 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-400'
                          : finding.status === 'resolved'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                          : 'bg-gray-100 text-gray-700 dark:bg-gray-900/20 dark:text-gray-400'
                      }`}>{finding.status}</span>
                    </td>
                    <td className="p-3 text-xs text-muted-foreground">
                      {new Date(finding.detected_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">Page {page} of {totalPages}</p>
              <div className="flex gap-2">
                <button onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1}
                  className="rounded-md border bg-background px-3 py-1.5 text-sm hover:bg-muted disabled:opacity-50">
                  Previous
                </button>
                <button onClick={() => setPage(Math.min(totalPages, page + 1))} disabled={page >= totalPages}
                  className="rounded-md border bg-background px-3 py-1.5 text-sm hover:bg-muted disabled:opacity-50">
                  Next
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function getMockFindings(): Finding[] {
  return [
    {
      id: '1',
      source: 'checkov',
      type: 'misconfiguration',
      severity: 'critical',
      title: 'CKV_AWS_111: S3 Bucket is publicly accessible',
      description: 'S3 Bucket checkout-logs is publicly accessible',
      asset_id: 'arn:aws:s3:::checkout-logs',
      status: 'open',
      detected_at: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: '2',
      source: 'trivy',
      type: 'vulnerability',
      severity: 'high',
      title: 'CVE-2026-1234: libssl 1.1.1 buffer overflow',
      description: 'Buffer overflow in libssl 1.1.1 allows remote code execution',
      asset_id: 'image:org/checkout:v2.3',
      status: 'open',
      detected_at: new Date(Date.now() - 7200000).toISOString(),
    },
    {
      id: '3',
      source: 'gitleaks',
      type: 'secret',
      severity: 'critical',
      title: 'Secret found: AWS Access Key',
      description: 'AWS Access Key ID found in config.js',
      asset_id: 'git:github.com/org/checkout-api',
      status: 'open',
      detected_at: new Date(Date.now() - 86400000).toISOString(),
    },
    {
      id: '4',
      source: 'k8s-custom',
      type: 'misconfiguration',
      severity: 'critical',
      title: 'Privileged container running',
      description: 'Pod checkout-api-7f8b9 has privileged containers',
      asset_id: 'k8s:production/pod/checkout-api-7f8b9',
      status: 'open',
      detected_at: new Date(Date.now() - 1800000).toISOString(),
    },
    {
      id: '5',
      source: 'trivy',
      type: 'vulnerability',
      severity: 'medium',
      title: 'CVE-2026-5678: log4j 2.14.1 DoS vulnerability',
      description: 'Denial of service in log4j 2.14.1',
      asset_id: 'image:org/checkout:v2.3',
      status: 'resolved',
      detected_at: new Date(Date.now() - 604800000).toISOString(),
    },
  ];
}
