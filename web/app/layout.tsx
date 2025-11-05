export const metadata = { title: "QOTD" };

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body style={{ fontFamily: 'system-ui, sans-serif', margin: 0, padding: 20, background: '#f9fafb' }}>
        <main style={{ maxWidth: 720, margin: '0 auto' }}>{children}</main>
      </body>
    </html>
  );
}

