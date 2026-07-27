'use client';

import { useEffect, useState, useRef } from 'react';
import { useAuth } from '@/components/auth-provider';
import { Bot, Send, Database, Clock, AlertCircle, Sparkles } from 'lucide-react';

interface QueryResult {
  question: string;
  cypher: string;
  results: Record<string, any>[];
  rows: number;
  duration_ms: number;
  error?: string;
  suggestions?: any[];
}

interface Template {
  id: string;
  question: string;
  description: string;
  category: string;
}

interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
  result?: QueryResult;
  timestamp: string;
}

const CATEGORY_COLORS: Record<string, string> = {
  Findings: 'text-red-500',
  Assets: 'text-blue-500',
  Risk: 'text-orange-500',
  'Attack Paths': 'text-purple-500',
  Dashboard: 'text-green-500',
  Identity: 'text-violet-500',
};

export default function AIAssistantPage() {
  const { isAuthenticated } = useAuth();
  const [messages, setMessages] = useState<ChatMessage[]>([
    { role: 'assistant', content: 'Hello! I am your security AI assistant. Ask me anything about your cloud security posture, or try one of the example questions below.', timestamp: new Date().toISOString() },
  ]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [activeCategory, setActiveCategory] = useState<string>('All');
  const chatEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isAuthenticated) return;
    fetchTemplates();
  }, [isAuthenticated]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const fetchTemplates = async () => {
    const token = localStorage.getItem('sentinel_token');
    try {
      const res = await fetch('http://localhost:8080/api/v1/ai/templates', {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (res.ok) {
        const data = await res.json();
        setTemplates(data.templates || []);
      }
    } catch {}
  };

  const ask = async (question: string) => {
    if (!question.trim()) return;

    setMessages((prev) => [...prev, { role: 'user', content: question, timestamp: new Date().toISOString() }]);
    setInput('');
    setLoading(true);

    const token = localStorage.getItem('sentinel_token');
    try {
      const res = await fetch('http://localhost:8080/api/v1/ai/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ question }),
      });

      const data = await res.json();

      if (data.error) {
        setMessages((prev) => [...prev, {
          role: 'assistant',
          content: `I couldn't answer that question. ${data.error}`,
          result: data,
          timestamp: new Date().toISOString(),
        }]);
      } else {
        setMessages((prev) => [...prev, {
          role: 'assistant',
          content: data.rows > 0
            ? `Found ${data.rows} result(s) in ${data.duration_ms}ms`
            : 'No results found.',
          result: data,
          timestamp: new Date().toISOString(),
        }]);
      }
    } catch {
      setMessages((prev) => [...prev, {
        role: 'assistant',
        content: 'Network error. Make sure the AI assistant service is running.',
        timestamp: new Date().toISOString(),
      }]);
    } finally {
      setLoading(false);
    }
  };

  const categories = ['All', ...new Set(templates.map((t) => t.category))];
  const filteredTemplates = activeCategory === 'All' ? templates : templates.filter((t) => t.category === activeCategory);

  if (!isAuthenticated) {
    return <div className="container py-6"><p className="text-muted-foreground">Please log in.</p></div>;
  }

  return (
    <div className="container py-6">
      <div className="flex gap-6 h-[calc(100vh-8rem)]">
        {/* Sidebar — Templates */}
        <div className="w-72 flex-shrink-0 space-y-4 overflow-y-auto">
          <div className="flex items-center gap-2">
            <Bot className="h-5 w-5 text-primary" />
            <h2 className="font-semibold">Example Questions</h2>
          </div>

          <div className="flex flex-wrap gap-1">
            {categories.map((cat) => (
              <button
                key={cat}
                onClick={() => setActiveCategory(cat)}
                className={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                  activeCategory === cat
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted hover:bg-muted/80'
                }`}
              >
                {cat}
              </button>
            ))}
          </div>

          <div className="space-y-1">
            {filteredTemplates.map((t) => (
              <button
                key={t.id}
                onClick={() => ask(t.question)}
                className="w-full text-left p-2 rounded-md hover:bg-muted/50 transition-colors text-sm"
              >
                <div className="flex items-center gap-2">
                  <Sparkles className={`h-3 w-3 flex-shrink-0 ${CATEGORY_COLORS[t.category] || ''}`} />
                  <span className="line-clamp-2">{t.question}</span>
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Chat Area */}
        <div className="flex-1 flex flex-col rounded-lg border bg-card overflow-hidden">
          <div className="p-3 border-b bg-muted/30">
            <div className="flex items-center gap-2">
              <Bot className="h-5 w-5 text-primary" />
              <span className="font-semibold">Security AI Assistant</span>
              <span className="text-xs text-muted-foreground ml-auto">Powered by Neo4j Graph</span>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            {messages.map((msg, idx) => (
              <div key={idx} className={`flex gap-3 ${msg.role === 'user' ? 'flex-row-reverse' : ''}`}>
                <div className={`flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold ${
                  msg.role === 'user'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted'
                }`}>
                  {msg.role === 'user' ? 'U' : 'AI'}
                </div>

                <div className={`flex-1 max-w-[80%] space-y-2 ${msg.role === 'user' ? 'text-right' : ''}`}>
                  <div className={`rounded-lg p-3 text-sm ${
                    msg.role === 'user' ? 'bg-primary text-primary-foreground' : 'bg-muted'
                  }`}>
                    {msg.content}
                  </div>

                  {msg.result && msg.result.cypher && (
                    <div className="rounded-md bg-muted/50 border p-2 text-xs font-mono text-muted-foreground">
                      <div className="flex items-center gap-1 mb-1">
                        <Database className="h-3 w-3" />
                        <span className="font-medium">Cypher</span>
                        <Clock className="h-3 w-3 ml-2" />
                        <span>{msg.result.duration_ms}ms</span>
                      </div>
                      <pre className="whitespace-pre-wrap break-all">{msg.result.cypher}</pre>
                    </div>
                  )}

                  {msg.result && msg.result.results && msg.result.results.length > 0 && (
                    <div className="rounded-md border overflow-hidden">
                      <table className="w-full text-xs">
                        <thead>
                          <tr className="bg-muted/50">
                            {Object.keys(msg.result.results[0]).map((key) => (
                              <th key={key} className="text-left p-2 font-medium">{key}</th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {msg.result.results.map((row, ri) => (
                            <tr key={ri} className="border-t border-muted/30">
                              {Object.values(row).map((val, vi) => (
                                <td key={vi} className="p-2 truncate max-w-[200px]">
                                  {formatCellValue(val)}
                                </td>
                              ))}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                      <div className="p-2 text-xs text-muted-foreground border-t bg-muted/20">
                        {msg.result.rows} row(s)
                      </div>
                    </div>
                  )}

                  {msg.result && msg.result.suggestions && (
                    <div className="rounded-md bg-orange-50 dark:bg-orange-900/10 border border-orange-200 dark:border-orange-900/20 p-2 text-xs">
                      <div className="flex items-center gap-1 text-orange-600 dark:text-orange-400 font-medium mb-1">
                        <AlertCircle className="h-3 w-3" />
                        Suggestions:
                      </div>
                      {msg.result.suggestions.map((s: any) => (
                        <button
                          key={s.id}
                          onClick={() => ask(s.question)}
                          className="block w-full text-left p-1 hover:bg-orange-100 dark:hover:bg-orange-900/20 rounded transition-colors"
                        >
                          {s.question}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
            <div ref={chatEndRef} />
          </div>

          <div className="p-3 border-t">
            <form
              onSubmit={(e) => { e.preventDefault(); ask(input); }}
              className="flex gap-2"
            >
              <input
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder="Ask a security question..."
                disabled={loading}
                className="flex-1 rounded-md border bg-background px-3 py-2 text-sm"
              />
              <button
                type="submit"
                disabled={loading || !input.trim()}
                className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
              >
                {loading ? '...' : <Send className="h-4 w-4" />}
              </button>
            </form>
          </div>
        </div>
      </div>
    </div>
  );
}

function formatCellValue(val: any): string {
  if (val === null || val === undefined) return '—';
  if (typeof val === 'object') return JSON.stringify(val).substring(0, 60);
  return String(val).substring(0, 60);
}
