import { NextResponse } from "next/server";

const DEFAULT_API_BASE = "http://localhost:8080";

export async function POST() {
  const cronKey = process.env.CRON_KEY;
  if (!cronKey) {
    return NextResponse.json({ error: "CRON_KEY not set" }, { status: 500 });
  }
  const apiBase = process.env.NEXT_PUBLIC_API_BASE || DEFAULT_API_BASE;
  try {
    const resp = await fetch(`${apiBase}/v1/admin/generate-today`, {
      method: "POST",
      headers: {
        "X-CRON-KEY": cronKey,
      },
      cache: "no-store",
    });
    const payload = await resp.json().catch(() => ({}));
    return NextResponse.json(payload, { status: resp.status });
  } catch (err) {
    return NextResponse.json({ error: (err as Error).message || "request failed" }, { status: 500 });
  }
}

export function GET() {
  return NextResponse.json({ error: "use POST" }, { status: 405 });
}
