export const pairCopy = {
  title: "Pair phone",
  helper: "Your phone must already be connected to Tailscale.",
  localhost: "You're on localhost - open the dashboard on its tailnet address before scanning.",
  buttonIdle: "Pair phone",
  buttonRefresh: "Show a new code",
  loading: "Preparing code",
  expiresIn: (seconds: number) => `Expires in ${formatCountdown(seconds)}`,
  expired: "Expired",
  urlLabel: "Pairing URL",
  fallback: "Use this URL if the camera cannot scan the QR.",
};

function formatCountdown(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}
