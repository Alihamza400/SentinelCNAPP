'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/components/auth-provider';
import { LogOut, User } from 'lucide-react';

const NAV_LINKS = [
  { href: '/', label: 'Dashboard' },
  { href: '/assets', label: 'Assets' },
  { href: '/findings', label: 'Findings' },
  { href: '/graph', label: 'Graph' },
  { href: '/attack-paths', label: 'Attack Paths' },
  { href: '/remediation', label: 'Remediation' },
  { href: '/ai-assistant', label: 'AI Assistant' },
];

export function SiteHeader() {
  const { isAuthenticated, isLoading, user, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  const isAuthPage = pathname === '/login' || pathname === '/signup';

  const handleLogout = () => {
    logout();
    router.push('/login');
  };

  return (
    <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
      <div className="container flex h-14 items-center gap-4">
        <Link href="/" className="flex shrink-0 items-center space-x-2">
          <span className="font-bold text-lg">SentinelCNAPP</span>
        </Link>

        {!isLoading && isAuthenticated && !isAuthPage && (
          <nav className="hidden md:flex flex-1 items-center space-x-6 text-sm font-medium">
            {NAV_LINKS.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={`transition-colors hover:text-foreground/80 ${
                  pathname === link.href
                    ? 'text-foreground'
                    : 'text-foreground/60'
                }`}
              >
                {link.label}
              </Link>
            ))}
          </nav>
        )}

        <div className="flex flex-1 items-center justify-end space-x-3">
          <span className="hidden sm:inline text-sm text-muted-foreground">
            v0.1.0
          </span>

          {isAuthenticated && !isLoading ? (
            <div className="flex items-center gap-3">
              <span className="hidden sm:flex items-center gap-1.5 text-sm text-muted-foreground">
                <User className="h-4 w-4" />
                {user?.email}
              </span>
              <button
                type="button"
                onClick={handleLogout}
                className="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-accent hover:text-accent-foreground"
              >
                <LogOut className="h-4 w-4" />
                Logout
              </button>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <Link
                href="/login"
                className="rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground hover:text-foreground"
              >
                Login
              </Link>
              <Link
                href="/signup"
                className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90"
              >
                Sign up
              </Link>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
