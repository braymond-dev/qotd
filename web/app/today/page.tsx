import ClientForm from "./ClientForm";

type Question = { id: string; title: string; text: string; topic: string; created_at: string };

async function getQuestion(apiBase: string): Promise<Question | null> {
  const res = await fetch(`${apiBase}/v1/question/today`, { cache: 'no-store' });
  if (!res.ok) return null;
  return res.json();
}

export default async function Page() {
  const apiBase = process.env.NEXT_PUBLIC_API_BASE || 'http://localhost:8080';
  const q = await getQuestion(apiBase);
  return (
    <div>
      <h1 style={{ fontSize: 28, margin: 0 }}>Question of the Day</h1>
      {!q ? (
        <p style={{ marginTop: 12 }}>No question yet. Ask admin to generate one.</p>
      ) : (
        <div style={{ marginTop: 12 }}>
          <div style={{ color: '#6b7280' }}>{q.topic}</div>
          <h2 style={{ margin: '6px 0 8px' }}>{q.title}</h2>
          <p style={{ background: '#fff', border: '1px solid #e5e7eb', padding: 12, borderRadius: 6 }}>{q.text}</p>
          <ClientForm apiBase={apiBase} questionId={q.id} />
        </div>
      )}
    </div>
  );
}
