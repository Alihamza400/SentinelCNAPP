import { AuthProvider } from '@/components/auth-provider';
import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'SentinelCNAPP',
  description: 'Unified Cloud-Native Application Protection Platform',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="dark">
      <body className={inter.className}>
        <AuthProvider>
          <div className="flex min-h-screen flex-col">
            <header className="sticky top-0 z-50 w-full border-b bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60">
              <div className="container flex h-14 items-center">
                <div className="mr-4 flex">
                  <a className="flex items-center space-x-2" href="/">
                    <span className="font-bold text-lg">SentinelCNAPP</span>
                  </a>
                </div>
                <nav className="flex items-center space-x-6 text-sm font-medium">
                  <a href="/" className="transition-colors hover:text-foreground/80 text-foreground/60">Dashboard</a>
                  <a href="/assets" className="transition-colors hover:text-foreground/80 text-foreground/60">Assets</a>
                  <a href="/findings" className="transition-colors hover:text-foreground/80 text-foreground/60">Findings</a>
                  <a href="/graph" className="transition-colors hover:text-foreground/80 text-foreground/60">Graph</a>
                  <a href="/attack-paths" className="transition-colors hover:text-foreground/80 text-foreground/60">Attack Paths</a>
                  <a href="/remediation" className="transition-colors hover:text-foreground/80 text-foreground/60">Remediation</a>
                  <a href="/ai-assistant" className="transition-colors hover:text-foreground/80 text-foreground/60">AI Assistant</a>
                </nav>
                <div className="flex flex-1 items-center justify-end space-x-4">
                  <span className="text-sm text-muted-foreground">v0.1.0</span>
                </div>
              </div>
            </header>
            <main className="flex-1">{children}</main>
          </div>
        </AuthProvider>
      </body>
    </html>
  );
}
