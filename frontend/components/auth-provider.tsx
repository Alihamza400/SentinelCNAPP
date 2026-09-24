'use client';

import { createContext, useContext, useEffect, useState, useCallback } from 'react';

interface User {
  id: string;
  email: string;
  name: string;
  roles: string[];
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  signup: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
  isAuthenticated: boolean;
  isLoading: boolean;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const storedToken = localStorage.getItem('sentinel_token');
    const storedUser = localStorage.getItem('sentinel_user');
    if (storedToken && storedUser) {
      setToken(storedToken);
      setUser(JSON.parse(storedUser));
    }
    setIsLoading(false);
  }, []);

  const persistSession = useCallback((u: User, t: string) => {
    setUser(u);
    setToken(t);
    localStorage.setItem('sentinel_token', t);
    localStorage.setItem('sentinel_user', JSON.stringify(u));
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    // TODO: Phase 1 - Replace with real OIDC flow via Dex
    // For development, accept hardcoded credentials
    const normalized = email.trim().toLowerCase();
    if (normalized === 'admin@sentinel-cnapp.io' && password === 'password') {
      persistSession(
        {
          id: '08a8684b-db88-4b73-90a9-3cd1661f5466',
          email: 'admin@sentinel-cnapp.io',
          name: 'Admin',
          roles: ['admin'],
        },
        'dev-mock-token-admin',
      );
      return;
    }
    if (normalized === 'viewer@sentinel-cnapp.io' && password === 'password') {
      persistSession(
        {
          id: '18a8684b-db88-4b73-90a9-3cd1661f5466',
          email: 'viewer@sentinel-cnapp.io',
          name: 'Viewer',
          roles: ['viewer'],
        },
        'dev-mock-token-viewer',
      );
      return;
    }

    // Allow any previously signed-up account (stored locally for demo)
    try {
      const raw = localStorage.getItem('sentinel_signup_users');
      const users: Record<string, { password: string; user: User }> = raw
        ? JSON.parse(raw)
        : {};
      const record = users[normalized];
      if (record && record.password === password) {
        persistSession(record.user, `dev-mock-token-${record.user.id}`);
        return;
      }
    } catch {
      // fall through
    }

    throw new Error('Invalid credentials');
  }, [persistSession]);

  const signup = useCallback(async (name: string, email: string, password: string) => {
    const normalized = email.trim().toLowerCase();
    if (!name.trim()) throw new Error('Name is required');
    if (!normalized.includes('@')) throw new Error('Valid email is required');
    if (password.length < 4) throw new Error('Password must be at least 4 characters');

    if (
      normalized === 'admin@sentinel-cnapp.io' ||
      normalized === 'viewer@sentinel-cnapp.io'
    ) {
      throw new Error('That email is already registered — try logging in');
    }

    let users: Record<string, { password: string; user: User }> = {};
    try {
      const raw = localStorage.getItem('sentinel_signup_users');
      users = raw ? JSON.parse(raw) : {};
    } catch {
      users = {};
    }

    if (users[normalized]) {
      throw new Error('That email is already registered — try logging in');
    }

    const mockUser: User = {
      id: crypto.randomUUID(),
      email: normalized,
      name: name.trim(),
      roles: ['user'],
    };
    users[normalized] = { password, user: mockUser };
    localStorage.setItem('sentinel_signup_users', JSON.stringify(users));

    persistSession(mockUser, `dev-mock-token-${mockUser.id}`);
  }, [persistSession]);

  const logout = useCallback(() => {
    setUser(null);
    setToken(null);
    localStorage.removeItem('sentinel_token');
    localStorage.removeItem('sentinel_user');
  }, []);

  return (
    <AuthContext.Provider value={{
      user, token, login, signup, logout,
      isAuthenticated: !!user,
      isLoading,
    }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
