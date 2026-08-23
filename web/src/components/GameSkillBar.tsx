import type { GameSkill } from "../api/client";

export function GameSkillBar({ skill }: { skill: GameSkill }) {
  const max = Math.max(skill.next_tier_xp, skill.tier_min + 1);
  const percent =
    skill.next_tier == null
      ? 100
      : Math.max(0, Math.min(100, ((skill.xp - skill.tier_min) / (max - skill.tier_min)) * 100));

  return (
    <div>
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="font-medium text-zinc-300">
          {skill.tier} {skill.xp}/{skill.next_tier_xp}
        </span>
        {skill.next_tier ? <span className="text-zinc-500">-&gt; {skill.next_tier}</span> : null}
      </div>
      <div
        className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/10"
        role="progressbar"
        aria-label={`${skill.name} XP tier`}
        aria-valuemin={skill.tier_min}
        aria-valuemax={max}
        aria-valuenow={skill.xp}
      >
        <div className="h-full rounded-full bg-cyan-300" style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}
