// API configuration — centralized for all frontend services
// In development: Envoy gateway runs on port 8081
// In production: Set NEXT_PUBLIC_API_URL environment variable

export const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081';
