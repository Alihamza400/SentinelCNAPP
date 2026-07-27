'use client';

import { useEffect, useState } from 'react';
import { useAuth } from '@/components/auth-provider';
import { Shield, CheckCircle2, XCircle, Clock, AlertTriangle } from 'lucide-react';

interface Remediation {
  id: string;
  finding_id: string;
  action_type: string;
  target_id: string;
  title: string;
  description: string;
  severity: string;
  status: string;
  auto_remediate: boolean;
  created_at: string;
}

const SEVERITY_STYLES: Record<string, string> = {
  critical: 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-400',
  high: 'bg-orange-100 text-orange-700 dark:bg-orange-900/20 dark:text-orange-400',
  medium: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400',
  low: 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400',
};

const STATUS_ICONS: Record<string, React.ReactNode> = {
  pending_approval: <Clock className="h-4 w-4 text-yellow-500" />,
  approved: <AlertTriangle className="h-4 w-4 text-blue-500" />,
  in_progress: <Clock className="h-4 w-4 text-blue-500" />,
  completed: <CheckCircle2 className="h-4 w-4 text-green-500" />,
  failed: <XCircle className="h-4 w-4 text-red-500" />,
  rejected: <XCircle className="h-4 w-4 text-gray-500" />,
};

export default function RemediationPage() {
  const { isAuthenticated, user } = useAuth();
  const [pending, setPending] = useState<Remediation[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionMsg, setActionMsg] = useState('');
  const [approvingId, setApprovingId] = useState<string | null>(null);

  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchPending = async () => {
      const token = localStorage.getItem('sentinel_token');
      try {
        const res = await fetch('http://localhost:8080/api/v1/remediation/pending', {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (res.ok) {
          const data = await res.json();
          setPending(data.remediations || []);
        } else {
          setPending(mockPending);
        }
      } catch {
        setPending(mockPending);
      } finally {
        setLoading(false);
      }
    };

    fetchPending();
  }, [isAuthenticated]);

  const approve = async (id: string) => {
    setApprovingId(id);
    setActionMsg('');

    const token = localStorage.getItem('sentinel_token');
    try {
      const res = await fetch(`http://localhost:8080/api/v1/remediation/approve/${id}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ approved_by: user?.email || 'admin' }),
      });

      if (res.ok) {
        setPending((prev) => prev.filter((r) => r.id !== id));
        setActionMsg(`✅ Remediation approved and executed successfully`);
      } else {
        const err = await res.json();
        setActionMsg(`❌ ${err.error || 'Failed to approve'}`);
      }
    } catch {
      setActionMsg('❌ Network error');
    } finally {
      setApprovingId(null);
    }
  };

  if (!isAuthenticated) {
    return <div className="container py-6"><p className="text-muted-foreground">Please log in.</p></div>;
  }

  return (
    <div className="container py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Remediation</h1>
          <p className="text-muted-foreground mt-1">
            {pending.length} actions pending approval
          </p>
        </div>
        <Shield className="h-8 w-8 text-green-500" />
      </div>

      {actionMsg && (
        <div className="rounded-md bg-muted p-3 text-sm">{actionMsg}</div>
      )}

      {loading ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <p className="text-muted-foreground">Loading...</p>
        </div>
      ) : pending.length === 0 ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <CheckCircle2 className="h-12 w-12 text-green-500 mx-auto mb-4" />
          <p className="text-lg font-medium">All caught up</p>
          <p className="text-muted-foreground">No remediation actions pending approval.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {pending.map((r) => (
            <div key={r.id} className="rounded-lg border bg-card p-4">
              <div className="flex items-start justify-between">
                <div className="space-y-1 flex-1">
                  <div className="flex items-center gap-2">
                    {STATUS_ICONS[r.status]}
                    <span className="font-medium">{r.title}</span>
                    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${SEVERITY_STYLES[r.severity] || ''}`}>
                      {r.severity}
                    </span>
                    {r.auto_remediate && (
                      <span className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-blue-100 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400">
                        auto
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground">{r.description}</p>
                  <div className="flex gap-4 text-xs text-muted-foreground">
                    <span>Type: {r.action_type}</span>
                    <span>Target: <code className="text-xs">{r.target_id}</code></span>
                  </div>
                </div>
                <div className="flex gap-2 ml-4">
                  <button
                    onClick={() => approve(r.id)}
                    disabled={approvingId === r.id}
                    className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                  >
                    {approvingId === r.id ? 'Approving...' : 'Approve'}
                  </button>
                  <button className="inline-flex items-center gap-1 rounded-md border bg-background px-3 py-1.5 text-xs font-medium hover:bg-muted">
                    Reject
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="rounded-lg border bg-card p-4">
        <h3 className="font-semibold text-sm mb-2">Remediation Actions Available</h3>
        <div className="text-xs text-muted-foreground space-y-1">
          <p><code>s3:block_public_access</code> — Enable BlockPublicAccess on S3 buckets</p>
          <p><code>s3:deny_public_policy</code> — Add explicit deny policy for public access</p>
          <p><code>iam:restrict_trust_policy</code> — Restrict IAM role trust policy</p>
          <p><code>ec2:revoke_public_ingress</code> — Remove 0.0.0.0/0 security group rules</p>
          <p><code>ecr:enable_scan_on_push</code> — Enable vulnerability scanning on push</p>
          <p><code>k8s:enforce_pod_security</code> — Apply pod security standards</p>
        </div>
      </div>
    </div>
  );
}

const mockPending: Remediation[] = [
  {
    id: 'rem-mock-1',
    finding_id: 'finding-1',
    action_type: 's3:block_public_access',
    target_id: 'arn:aws:s3:::checkout-logs',
    title: 'Block S3 Bucket Public Access',
    description: 'Enable BlockPublicAccess settings on the S3 bucket to prevent public access',
    severity: 'critical',
    status: 'pending_approval',
    auto_remediate: false,
    created_at: new Date().toISOString(),
  },
  {
    id: 'rem-mock-2',
    finding_id: 'finding-2',
    action_type: 'ec2:revoke_public_ingress',
    target_id: 'arn:aws:ec2:us-east-1:123456789012:security-group/sg-abc123',
    title: 'Remove Public Ingress Rules',
    description: 'Remove 0.0.0.0/0 ingress rules from the security group',
    severity: 'high',
    status: 'pending_approval',
    auto_remediate: false,
    created_at: new Date().toISOString(),
  },
];
