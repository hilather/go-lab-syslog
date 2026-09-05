/** Render untrusted syslog bytes as text. Never HTML. */
export function TextBlock({ text, className }: { text: string; className?: string }) {
  return (
    <pre className={className} data-testid="text-block">
      {text}
    </pre>
  );
}
