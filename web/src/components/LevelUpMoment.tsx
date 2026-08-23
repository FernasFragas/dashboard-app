import { Trophy } from "lucide-react";

import type { GameLevelUp } from "../api/client";
import { gameCopy } from "../copy/game";
import { usePrefersReducedMotion } from "../lib/motion";

export function LevelUpMoment({ levelUp, onClose }: { levelUp: GameLevelUp; onClose: () => void }) {
  const reducedMotion = usePrefersReducedMotion();

  return (
    <button
      type="button"
      className="fixed inset-0 z-50 grid place-items-center bg-black/80 p-6 text-center"
      onClick={onClose}
    >
      <span
        className={[
          "flex max-w-md flex-col items-center gap-3 rounded-md border border-cyan-300/40 bg-zinc-950 px-6 py-8 text-white shadow-2xl shadow-cyan-950/40",
          reducedMotion ? "" : "animate-pulse",
        ].join(" ")}
      >
        <Trophy className="size-10 text-amber-200" aria-hidden="true" />
        <span className="text-sm font-semibold uppercase text-cyan-200">{gameCopy.level}</span>
        <span className="text-3xl font-semibold">
          {levelUp.level} · {levelUp.title}
        </span>
      </span>
    </button>
  );
}
