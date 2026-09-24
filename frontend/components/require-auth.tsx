'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/components/auth-provider';
import { Shield } from 'lucide-react';

/**
 * Client-side auth gate. Redirects unauthenticated users to /login
 * (preserving ?next= so they land back on the intended page).
 */
export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated) {
      const next = encodeURIComponent(window.location.pathname + window.location.search);
      router.replace(`/login?next=${next}`);
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <div className="flex flex-col items-center gap-3 text-muted-foreground">
          <Shield className="h-8 w-8 animate-pulse" />
          <p className="text-sm">Loading…</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <div className="flex flex-col items-center gap-3 text-center">
          <Shield className="h-8 w-8 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">Redirecting to login…</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
