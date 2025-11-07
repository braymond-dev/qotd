"use client";
import { useEffect, useMemo, useState } from "react";
import DevRefreshButton from "./DevRefreshButton";

type Props = { apiBase: string; questionId: string };

export default function ClientForm({ apiBase, questionId }: Props) {
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ score: number; feedback: string } | null>(null);
  const [celebrate, setCelebrate] = useState(false);

  useEffect(() => {
    if (!celebrate) return;
    const id = setTimeout(() => setCelebrate(false), 1500);
    return () => clearTimeout(id);
  }, [celebrate]);

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
      const score = data.score ?? 0;
      setResult({ score, feedback: data.feedback ?? '' });
      setCelebrate(score > 0);
    } catch (err: any) {
      setError(err.message || 'Submit failed');
      setCelebrate(false);
    } finally {
      setLoading(false);
    }
  }

  const sparks = useMemo(() =>
    Array.from({ length: 7 }).map((_, idx) => {
      const angle = (idx / 7) * Math.PI * 2;
      return {
        delay: idx * 0.08,
        dx: Math.cos(angle) * 80,
        dy: Math.sin(angle) * 50,
      };
    }), []);

  return (
    <form onSubmit={onSubmit} style={{ marginTop: 16 }}>
      <div className={`answer-wrapper${celebrate ? ' celebrate' : ''}`}>
        {celebrate && (
          <div className="spark-layer">
            {sparks.map(({ delay, dx, dy }, idx) => (
              <span
                key={idx}
                className="spark"
                style={{
                  animationDelay: `${delay}s`,
                  // @ts-ignore custom vars
                  '--dx': `${dx}px`,
                  '--dy': `${dy}px`,
                }}
              />
            ))}
          </div>
        )}
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={6}
          placeholder="Your answer..."
          className="answer-input"
        />
        <style jsx>{`
          .answer-wrapper {
            position: relative;
          }
          .answer-input {
            width: 100%;
            padding: 12px;
            resize: vertical;
            border-radius: 6px;
            border: 1px solid #d1d5db;
            transition: border-color 0.2s, box-shadow 0.2s;
          }
          .answer-wrapper.celebrate .answer-input {
            border-color: #22c55e;
            box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.2);
          }
          .spark-layer {
            position: absolute;
            inset: 0;
            pointer-events: none;
            z-index: 2;
          }
          .spark {
            position: absolute;
            top: 50%;
            left: 50%;
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: radial-gradient(circle, rgba(134, 239, 172, 0.95), rgba(34, 197, 94, 0.8));
            box-shadow: 0 0 8px rgba(34, 197, 94, 0.8);
            transform: translate(-50%, -50%);
            animation: spark 1s ease-out forwards;
          }
          @keyframes spark {
            0% {
              opacity: 0.9;
              transform: translate(-50%, -50%) scale(0.3);
            }
            70% {
              opacity: 1;
            }
            100% {
              opacity: 0;
              transform: translate(calc(-50% + var(--dx)), calc(-50% + var(--dy))) scale(1.2);
            }
          }
        `}</style>
      </div>
      <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginTop: 8, flexWrap: 'wrap' }}>
        <button type="submit" disabled={loading || text.trim().length === 0} style={{ padding: '8px 12px' }}>
          {loading ? 'Grading...' : 'Submit'}
        </button>
        <DevRefreshButton />
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
