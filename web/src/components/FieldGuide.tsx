import { Info } from "lucide-react";
import type { ReactNode } from "react";
import { useState } from "react";

export function FieldGuide({ label, children }: { label: string; children: ReactNode }) {
  const [open, setOpen] = useState(false);

  return (
    <div className="flex flex-col items-end gap-2">
      <button
        type="button"
        aria-label={label}
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-md border border-white/10 bg-zinc-950 text-zinc-300 outline-none hover:border-cyan-300 hover:text-cyan-200 focus:border-cyan-300"
      >
        <Info className="size-4" aria-hidden="true" />
      </button>

      {open ? (
        <div className="max-w-xl rounded-md border border-white/10 bg-zinc-950 px-3 py-2 text-sm leading-6 text-zinc-300 shadow-lg shadow-black/20">
          {children}
        </div>
      ) : null}
    </div>
  );
}
