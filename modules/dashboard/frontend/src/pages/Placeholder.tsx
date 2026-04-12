export function Placeholder({ title }: { title: string }) {
  return (
    <div className="bg-surface-container p-8 mt-8 text-center">
      <h1 className="text-lg font-semibold mb-4">{title}</h1>
      <p className="label-xs mb-2">Plan 3</p>
      <p className="font-mono text-sm text-on-surface-variant">
        This view is tracked but not yet implemented.
      </p>
    </div>
  );
}
