'use client';

import { useEffect, useState } from 'react';
import { useAuth } from '@/components/auth-provider';
import { RequireAuth } from '@/components/require-auth';
import { API_URL } from '@/lib/config';
import { Shield, ShieldAlert, ArrowRight,ExternalLink } from 'lucide-react';

interface AttackStep {
  node_id: string;
  node_type: string;
  label: string;
  name: string;
  detail: string;
  severity?: string;
}

interface AttackPath {
  id: string;
  risk_score: number;
  steps: AttackStep[];
  total_steps: number;
  max_severity: string;
  discovered_at: string;
}

interface AttackPathSummary {
  total_paths: number;
  unique_findings: number;
  unique_assets: number;
  exposed_resources: number;
  by_severity: Record<string, number>;
  top_paths: AttackPath[];
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-400',
  high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/20 dark:text-orange-400',
  medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400',
  low: 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400',
};

const STEP_ICONS: Record<string, string> = {
  Finding: '🔴',
  Asset: '🖥️',
  Identity: '🔑',
  Resource: '🎯',
};

function AttackPathsContent() {
  const { isAuthenticated } = useAuth();
  const [summary, setSummary] = useState<AttackPathSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [expandedPath, setExpandedPath] = useState<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchData = async () => {
      setLoading(true);
      const token = localStorage.getItem('sentinel_token');
      const headers = { Authorization: `Bearer ${token}` };

      try {
        const res = await fetch(`${API_URL}/api/v1/attack-paths/summary`, { headers });
        if (res.ok) {
          const data: AttackPathSummary = await res.json();
          setSummary(data);
        } else {
          setSummary(getMockSummary());
        }
      } catch {
        setSummary(getMockSummary());
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [isAuthenticated]);

  if (!isAuthenticated) {
    return <div className="container py-6"><p className="text-muted-foreground">Please log in to view attack paths.</p></div>;
  }

  if (loading) {
    return <div className="container py-6"><p className="text-muted-foreground">Loading attack path analysis...</p></div>;
  }

  const paths = summary?.top_paths || [];

  return (
    <div className="container py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Attack Paths</h1>
          <p className="text-muted-foreground mt-1">
            {summary ? `${summary.total_paths} paths from ${summary.unique_findings} findings` : 'Attack chain analysis'}
          </p>
        </div>
        <ShieldAlert className="h-8 w-8 text-orange-500" />
      </div>

      {summary && (
        <div className="grid gap-4 md:grid-cols-4">
          <div className="rounded-lg border bg-card p-4">
            <p className="text-sm text-muted-foreground">Total Paths</p>
            <p className="text-2xl font-bold">{summary.total_paths}</p>
          </div>
          <div className="rounded-lg border bg-card p-4">
            <p className="text-sm text-muted-foreground">Unique Findings</p>
            <p className="text-2xl font-bold">{summary.unique_findings}</p>
          </div>
          <div className="rounded-lg border bg-card p-4">
            <p className="text-sm text-muted-foreground">Affected Assets</p>
            <p className="text-2xl font-bold">{summary.unique_assets}</p>
          </div>
          <div className="rounded-lg border bg-card p-4">
            <p className="text-sm text-muted-foreground">Exposed Resources</p>
            <p className="text-2xl font-bold">{summary.exposed_resources}</p>
          </div>
        </div>
      )}

      {paths.length === 0 ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <Shield className="h-12 w-12 text-green-500 mx-auto mb-4" />
          <p className="text-lg font-medium">No attack paths found</p>
          <p className="text-muted-foreground">No exploitable chains from findings to exposed resources detected.</p>
        </div>
      ) : (
        <div className="space-y-4">
          {paths.map((path) => (
            <div key={path.id} className="rounded-lg border bg-card overflow-hidden">
              <button
                onClick={() => setExpandedPath(expandedPath === path.id ? null : path.id)}
                className="w-full p-4 flex items-center justify-between hover:bg-muted/50 transition-colors"
              >
                <div className="flex items-center gap-3">
                  <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_COLORS[path.max_severity] || SEVERITY_COLORS.low}`}>
                    {path.max_severity}
                  </span>
                  <span className="font-medium">
                    Risk Score: <span className={path.risk_score >= 7 ? 'text-red-500' : path.risk_score >= 4 ? 'text-orange-500' : ''}>
                      {path.risk_score.toFixed(1)}
                    </span>
                  </span>
                  <span className="text-sm text-muted-foreground">
                    {path.total_steps} steps
                  </span>
                </div>
                <ArrowRight className={`h-4 w-4 transition-transform ${expandedPath === path.id ? 'rotate-90' : ''}`} />
              </button>

              {expandedPath === path.id && (
                <div className="px-4 pb-4">
                  <div className="flex items-center gap-2 flex-wrap">
                    {path.steps.map((step, idx) => (
                      <div key={step.node_id} className="flex items-center gap-2">
                        <div className="rounded-lg border bg-muted/30 p-2 text-xs max-w-[200px]">
                          <div className="flex items-center gap-1 mb-1">
                            <span>{STEP_ICONS[step.node_type] || '•'}</span>
                            <span className="font-medium">{step.node_type}</span>
                          </div>
                          <p className="truncate font-medium">{step.name || step.node_id.split('/').pop()}</p>
                          <p className="text-muted-foreground truncate">{step.label}</p>
                          {step.severity && (
                            <span className={`inline-flex items-center rounded-full px-1.5 py-0.5 text-xs font-medium mt-1 ${SEVERITY_COLORS[step.severity] || ''}`}>
                              {step.severity}
                            </span>
                          )}
                        </div>
                        {idx < path.steps.length - 1 && (
                          <ArrowRight className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function getMockSummary(): AttackPathSummary {
  return {
    total_paths: 3,
    unique_findings: 2,
    unique_assets: 3,
    exposed_resources: 2,
    by_severity: { critical: 1, high: 2 },
    top_paths: [
      {
        id: 'path-1',
        risk_score: 8.5,
        max_severity: 'critical',
        total_steps: 4,
        discovered_at: new Date().toISOString(),
        steps: [
          { node_id: 'finding-1', node_type: 'Finding', label: 'Security Finding', name: 'S3 Bucket Publicly Accessible', detail: 'Severity: critical, Risk: 8.5', severity: 'critical' },
          { node_id: 'arn:aws:s3:::checkout-logs', node_type: 'Asset', label: 's3_bucket', name: 'checkout-logs', detail: 'Internet facing: true' },
          { node_id: 'arn:aws:iam::123456789012:role/checkout-role', node_type: 'Identity', label: 'IAM Role', name: 'checkout-role', detail: 'Over-privileged identity' },
          { node_id: 's3:checkout-logs', node_type: 'Resource', label: 's3_bucket', name: 'checkout-logs', detail: 'Internet facing: true' },
        ],
      },
      {
        id: 'path-2',
        risk_score: 7.2,
        max_severity: 'high',
        total_steps: 4,
        discovered_at: new Date().toISOString(),
        steps: [
          { node_id: 'finding-2', node_type: 'Finding', label: 'Security Finding', name: 'CVE-2026-1234: libssl buffer overflow', detail: 'Severity: high, Risk: 7.2', severity: 'high' },
          { node_id: 'arn:aws:ec2:us-east-1:i-abc123', node_type: 'Asset', label: 'ec2_instance', name: 'checkout-api-server', detail: 'Internet facing: true' },
          { node_id: 'arn:aws:iam::123456789012:role/checkout-role', node_type: 'Identity', label: 'IAM Role', name: 'checkout-role', detail: 'Over-privileged identity' },
          { node_id: 's3:checkout-logs', node_type: 'Resource', label: 's3_bucket', name: 'checkout-logs', detail: 'Internet facing: true' },
        ],
      },
    ],
  };
}

export default function AttackPathsPage() {
  return (
    <RequireAuth>
      <AttackPathsContent />
    </RequireAuth>
  );
}
