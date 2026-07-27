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

  const login = useCallback(async (email: string, password: string) => {
    // TODO: Phase 1 - Replace with real OIDC flow via Dex
    // For development, accept hardcoded credentials
    if (email === 'admin@sentinel-cnapp.io' && password === 'password') {
      const mockUser: User = {
        id: '08a8684b-db88-4b73-90a9-3cd1661f5466',
        email: 'admin@sentinel-cnapp.io',
        name: 'Admin',
        roles: ['admin'],
      };
      const mockToken = 'dev-mock-token-admin';
      setUser(mockUser);
      setToken(mockToken);
      localStorage.setItem('sentinel_token', mockToken);
      localStorage.setItem('sentinel_user', JSON.stringify(mockUser));
      return;
    }
    if (email === 'viewer@sentinel-cnapp.io' && password === 'password') {
      const mockUser: User = {
        id: '18a8684b-db88-4b73-90a9-3cd1661f5466',
        email: 'viewer@sentinel-cnapp.io',
        name: 'Viewer',
        roles: ['viewer'],
      };
      const mockToken = 'dev-mock-token-viewer';
      setUser(mockUser);
      setToken(mockToken);
      localStorage.setItem('sentinel_token', mockToken);
      localStorage.setItem('sentinel_user', JSON.stringify(mockUser));
      return;
    }
    throw new Error('Invalid credentials');
  }, []);

  const logout = useCallback(() => {
    setUser(null);
    setToken(null);
    localStorage.removeItem('sentinel_token');
    localStorage.removeItem('sentinel_user');
  }, []);

  return (
    <AuthContext.Provider value={{
      user, token, login, logout,
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
