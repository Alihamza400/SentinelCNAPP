'use client';

import { useEffect, useState } from 'react';
import { useAuth } from '@/components/auth-provider';
import { RequireAuth } from '@/components/require-auth';
import { listAssets, APIError } from '@/lib/api-client';
import { Cloud, Search, Filter, RefreshCw } from 'lucide-react';

interface Asset {
  id: string;
  provider: string;
  asset_type: string;
  name: string;
  region: string;
  environment: string;
  internet_facing: boolean;
  active: boolean;
  discovered_at: string;
}

const ASSET_TYPES = [
  'ec2_instance',
  'eks_cluster',
  's3_bucket',
  'iam_role',
  'iam_user',
  'lambda_function',
  'ecr_repository',
  'rds_instance',
  'security_group',
  'vpc',
  'ebs_volume',
];

const TYPE_LABELS: Record<string, string> = {
  ec2_instance: 'EC2 Instance',
  eks_cluster: 'EKS Cluster',
  s3_bucket: 'S3 Bucket',
  iam_role: 'IAM Role',
  iam_user: 'IAM User',
  lambda_function: 'Lambda Function',
  ecr_repository: 'ECR Repository',
  rds_instance: 'RDS Instance',
  security_group: 'Security Group',
  vpc: 'VPC',
  ebs_volume: 'EBS Volume',
};

function AssetsContent() {
  const { isAuthenticated } = useAuth();
  const [assets, setAssets] = useState<Asset[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 20;

  useEffect(() => {
    if (!isAuthenticated) return;

    const fetchAssets = async () => {
      setLoading(true);
      setError('');
      try {
        const result = await listAssets({
          type: typeFilter || undefined,
          search: search || undefined,
          page,
          page_size: pageSize,
        });
        setAssets(result.assets);
        setTotal(result.total);
      } catch (err) {
        if (err instanceof APIError) {
          setError(err.message);
        } else {
          setError('Failed to load assets');
        }
      } finally {
        setLoading(false);
      }
    };

    const timeout = setTimeout(fetchAssets, 300);
    return () => clearTimeout(timeout);
  }, [isAuthenticated, search, typeFilter, page]);

  if (!isAuthenticated) {
    return (
      <div className="container py-6">
        <p className="text-muted-foreground">Please log in to view assets.</p>
      </div>
    );
  }

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="container py-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Assets</h1>
          <p className="text-muted-foreground mt-1">{total} total assets</p>
        </div>
        <Cloud className="h-8 w-8 text-muted-foreground" />
      </div>

      <div className="flex items-center gap-4">
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search assets..."
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
            className="w-full rounded-md border bg-background pl-9 pr-3 py-2 text-sm"
          />
        </div>

        <select
          value={typeFilter}
          onChange={(e) => { setTypeFilter(e.target.value); setPage(1); }}
          className="rounded-md border bg-background px-3 py-2 text-sm"
        >
          <option value="">All Types</option>
          {ASSET_TYPES.map((type) => (
            <option key={type} value={type}>{TYPE_LABELS[type] || type}</option>
          ))}
        </select>

        <button
          onClick={() => { setPage(1); setSearch(''); setTypeFilter(''); }}
          className="rounded-md border bg-background px-3 py-2 text-sm hover:bg-muted"
        >
          <RefreshCw className="h-4 w-4" />
        </button>
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {loading ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <p className="text-muted-foreground">Loading assets...</p>
        </div>
      ) : assets.length === 0 ? (
        <div className="rounded-lg border bg-card p-12 text-center">
          <p className="text-muted-foreground">
            No assets found. Run a discovery sync to populate assets.
          </p>
        </div>
      ) : (
        <>
          <div className="rounded-lg border bg-card overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="text-left p-3 font-medium">Name</th>
                  <th className="text-left p-3 font-medium">Type</th>
                  <th className="text-left p-3 font-medium">Region</th>
                  <th className="text-left p-3 font-medium">Environment</th>
                  <th className="text-left p-3 font-medium">Exposed</th>
                  <th className="text-left p-3 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {assets.map((asset) => (
                  <tr key={asset.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                    <td className="p-3 font-medium">{asset.name || asset.id.split('/').pop()}</td>
                    <td className="p-3">{TYPE_LABELS[asset.asset_type] || asset.asset_type}</td>
                    <td className="p-3 text-muted-foreground">{asset.region}</td>
                    <td className="p-3">
                      <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                        asset.environment === 'production'
                          ? 'bg-red-100 text-red-700 dark:bg-red-900/20 dark:text-red-400'
                          : asset.environment === 'staging'
                          ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/20 dark:text-yellow-400'
                          : 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                      }`}>
                        {asset.environment}
                      </span>
                    </td>
                    <td className="p-3">
                      {asset.internet_facing ? (
                        <span className="text-red-500 font-medium">Yes</span>
                      ) : (
                        <span className="text-muted-foreground">No</span>
                      )}
                    </td>
                    <td className="p-3">
                      <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                        asset.active
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/20 dark:text-green-400'
                          : 'bg-gray-100 text-gray-700 dark:bg-gray-900/20 dark:text-gray-400'
                      }`}>
                        {asset.active ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <p className="text-sm text-muted-foreground">
                Page {page} of {totalPages}
              </p>
              <div className="flex gap-2">
                <button
                  onClick={() => setPage(Math.max(1, page - 1))}
                  disabled={page <= 1}
                  className="rounded-md border bg-background px-3 py-1.5 text-sm hover:bg-muted disabled:opacity-50"
                >
                  Previous
                </button>
                <button
                  onClick={() => setPage(Math.min(totalPages, page + 1))}
                  disabled={page >= totalPages}
                  className="rounded-md border bg-background px-3 py-1.5 text-sm hover:bg-muted disabled:opacity-50"
                >
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

export default function AssetsPage() {
  return (
    <RequireAuth>
      <AssetsContent />
    </RequireAuth>
  );
}
