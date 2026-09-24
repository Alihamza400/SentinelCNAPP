'use client';

import { Cloud, Search, AlertTriangle, Activity, CheckCircle2 } from 'lucide-react';
import { useAuth } from '@/components/auth-provider';
import { RequireAuth } from '@/components/require-auth';
import { useEffect, useState } from 'react';
import { listAssets } from '@/lib/api-client';
import { API_URL } from '@/lib/config';

interface DashboardStats {
  total_assets: number;
  open_findings: number;
  critical_findings: number;
}

interface SeverityDistribution {
  critical?: number;
  high?: number;
  medium?: number;
  low?: number;
  info?: number;
}

const scanners = [
  { name: 'Asset Inventory', engine: 'AWS SDK', status: 'active', phase: 'Phase 1' },
  { name: 'IaC Scanner', engine: 'Checkov', status: 'active', phase: 'Phase 2' },
  { name: 'Container Scanner', engine: 'Trivy', status: 'active', phase: 'Phase 2' },
  { name: 'K8s Scanner', engine: 'Trivy + Custom', status: 'active', phase: 'Phase 2' },
  { name: 'Secrets Scanner', engine: 'Gitleaks', status: 'active', phase: 'Phase 2' },
  { name: 'Correlation Graph', engine: 'Neo4j', status: 'active', phase: 'Phase 3' },
  { name: 'Runtime Protection', engine: 'Falco', status: 'planned', phase: 'Phase 4' },
];

function DashboardContent() {
  const { isAuthenticated } = useAuth();
  const [stats, setStats] = useState<DashboardStats>({ total_assets: 0, open_findings: 0, critical_findings: 0 });
  const [severityDist, setSeverityDist] = useState<SeverityDistribution>({});

  useEffect(() => {
    if (!isAuthenticated) return;

    // Fetch from asset inventory
    listAssets({ page_size: 1 }).then((res) => {
      setStats(prev => ({ ...prev, total_assets: res.total }));
    }).catch(() => {});

    // Fetch from correlation service
    const fetchCorrelation = async () => {
      const token = localStorage.getItem('sentinel_token');
      const headers = { Authorization: `Bearer ${token}` };

      try {
        const res = await fetch(`${API_URL}/api/v1/dashboard/stats`, { headers });
        if (res.ok) {
          const data: DashboardStats = await res.json();
          setStats(data);
        }
      } catch {}

      try {
        const res = await fetch(`${API_URL}/api/v1/dashboard/severity-distribution`, { headers });
        if (res.ok) {
          const data: SeverityDistribution = await res.json();
          setSeverityDist(data);
        }
      } catch {}
    };

    fetchCorrelation();
  }, [isAuthenticated]);

  const statCards = [
    { label: 'Assets Discovered', value: String(stats.total_assets), icon: Cloud, color: 'text-blue-500' },
    { label: 'Open Findings', value: String(stats.open_findings), icon: AlertTriangle, color: 'text-red-500' },
    { label: 'Critical', value: String(stats.critical_findings), icon: Activity, color: 'text-orange-500' },
    { label: 'Scanners Active', value: '6', icon: Search, color: 'text-green-500' },
  ];

  const totalBySeverity = Object.values(severityDist).reduce((a, b) => a + b, 0);

  return (
    <div className="container py-6 space-y-8">
      <div>
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="text-muted-foreground mt-1">Unified cloud security overview</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {statCards.map((stat) => (
          <div key={stat.label} className="rounded-lg border bg-card p-6">
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium text-muted-foreground">{stat.label}</p>
              <stat.icon className={`h-5 w-5 ${stat.color}`} />
            </div>
            <p className="text-3xl font-bold mt-2">{stat.value}</p>
          </div>
        ))}
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* Severity Distribution */}
        <div className="rounded-lg border bg-card">
          <div className="p-4 border-b">
            <h2 className="font-semibold">Finding Severity Distribution</h2>
          </div>
          <div className="p-4">
            {totalBySeverity > 0 ? (
              <div className="space-y-2">
                {[
                  { key: 'critical', label: 'Critical', color: 'bg-red-500' },
                  { key: 'high', label: 'High', color: 'bg-orange-500' },
                  { key: 'medium', label: 'Medium', color: 'bg-yellow-500' },
                  { key: 'low', label: 'Low', color: 'bg-green-500' },
                  { key: 'info', label: 'Info', color: 'bg-gray-500' },
                ].map((sev) => {
                  const count = (severityDist as any)[sev.key] || 0;
                  if (count === 0) return null;
                  const pct = (count / totalBySeverity) * 100;
                  return (
                    <div key={sev.key} className="space-y-1">
                      <div className="flex justify-between text-sm">
                        <span>{sev.label}</span>
                        <span className="text-muted-foreground">{count}</span>
                      </div>
                      <div className="h-2 rounded-full bg-muted overflow-hidden">
                        <div className={`h-full rounded-full ${sev.color}`} style={{ width: `${pct}%` }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground text-center py-4">
                No findings yet. Run scans to populate findings.
              </p>
            )}
          </div>
        </div>

        {/* Scanner Status */}
        <div className="rounded-lg border bg-card">
          <div className="p-4 border-b">
            <h2 className="font-semibold">Scanner Status</h2>
          </div>
          <div className="p-4">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b">
                  <th className="text-left py-2 font-medium">Scanner</th>
                  <th className="text-left py-2 font-medium">Engine</th>
                  <th className="text-left py-2 font-medium">Status</th>
                  <th className="text-left py-2 font-medium">Phase</th>
                </tr>
              </thead>
              <tbody>
                {scanners.map((scanner) => (
                  <tr key={scanner.name} className="border-b last:border-0">
                    <td className="py-2">{scanner.name}</td>
                    <td className="py-2 text-muted-foreground">{scanner.engine}</td>
                    <td className="py-2">
                      <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${
                        scanner.status === 'active'
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                          : scanner.status === 'development'
                          ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400'
                          : 'bg-gray-100 text-gray-700 dark:bg-gray-900/20 dark:text-gray-400'
                      }`}>
                        {scanner.status === 'active' && <CheckCircle2 className="h-3 w-3" />}
                        {scanner.status}
                      </span>
                    </td>
                    <td className="py-2 text-muted-foreground">{scanner.phase}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function Dashboard() {
  return (
    <RequireAuth>
      <DashboardContent />
    </RequireAuth>
  );
}
