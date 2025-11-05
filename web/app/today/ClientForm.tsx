"use client";
import { useState } from "react";

type Props = { apiBase: string; questionId: string };

export default function ClientForm({ apiBase, questionId }: Props) {
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ score: number; feedback: string } | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const res = await fetch(`${apiBase}/v1/answers`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ question_id: questionId, text }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setResult({ score: data.score ?? 0, feedback: data.feedback ?? '' });
    } catch (err: any) {
      setError(err.message || 'Submit failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <form onSubmit={onSubmit} style={{ marginTop: 16 }}>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={6}
        placeholder="Your answer..."
        style={{ width: '100%', padding: 12, resize: 'vertical' }}
      />
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 8 }}>
        <button type="submit" disabled={loading || text.trim().length === 0} style={{ padding: '8px 12px' }}>
          {loading ? 'Grading...' : 'Submit'}
        </button>
        {error && <span style={{ color: 'crimson' }}>{error}</span>}
      </div>
      {result && (
        <div style={{ marginTop: 12, padding: 12, background: '#fff', border: '1px solid #e5e7eb', borderRadius: 6 }}>
          <div><strong>Score:</strong> {result.score}</div>
          <div style={{ marginTop: 6 }}><strong>Feedback:</strong> {result.feedback}</div>
        </div>
      )}
    </form>
  );
}

