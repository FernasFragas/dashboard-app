export const gameCopy = {
  quest: "Quest",
  quests: "Quests",
  chapter: "Chapter",
  bossReview: "Boss review",
  questReport: "Quest report",
  level: "Level",
  nextLevel: "Next level",
  badges: "Badges",
  skills: "Skills",
  xp: "XP",
  noBadges: "No badges unlocked yet",
  profile: "Profile",
  shielded: "shielded",
};

export function chapterLabel(code: string, focus?: string | null): string {
  return `${gameCopy.chapter} ${code}${focus ? ` · ${focus}` : ""}`;
}

export function gameToast(game: {
  xp_awarded: number;
  unlocks?: { name: string }[];
  event?: { source_label?: string; skill_names?: string[] };
}): string | null {
  if (game.xp_awarded > 0) {
    const source = game.event?.source_label?.replace(" - ", " · ");
    const skills = game.event?.skill_names?.join(", ");
    return [`+${game.xp_awarded} XP`, source, skills].filter(Boolean).join(" · ");
  }

  if (game.unlocks && game.unlocks.length > 0) {
    return `Badge unlocked · ${game.unlocks[0].name}`;
  }

  return null;
}
