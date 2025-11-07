"use client";

import { useState } from "react";

export default function DevRefreshButton() {
  const [status, setStatus] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const onClick = async () => {
    setLoading(true);
    setStatus(null);
    try {
      const resp = await fetch("/api/dev/refresh", { method: "POST" });
      if (!resp.ok) {
        const body = await resp.json().catch(() => ({}));
        throw new Error(body?.error || `HTTP ${resp.status}`);
      }
      setStatus("New question requested");
      window.location.reload();
    } catch (err: any) {
      setStatus(err?.message || "Refresh failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ marginTop: 12 }}>
      <button onClick={onClick} disabled={loading} style={{ padding: "6px 10px" }}>
        {loading ? "Refreshing…" : "Refresh question"}
      </button>
      {status && <span style={{ marginLeft: 8, color: status.includes("failed") ? "crimson" : "#047857" }}>{status}</span>}
    </div>
  );
}
