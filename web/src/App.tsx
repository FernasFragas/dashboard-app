import { useEffect, useState } from "react";

import { getHealth, type HealthResponse } from "./api/client";

type HealthState =
  | { status: "loading" }
  | { status: "ready"; health: HealthResponse }
  | { status: "error"; message: string };

export default function App() {
  const [healthState, setHealthState] = useState<HealthState>({ status: "loading" });

  useEffect(() => {
    let ignore = false;

    async function loadHealth() {
      try {
        const health = await getHealth();
        if (!ignore) {
          setHealthState({ status: "ready", health });
        }
      } catch (error) {
        if (!ignore) {
          setHealthState({
            status: "error",
            message: error instanceof Error ? error.message : "health check failed",
          });
        }
      }
    }

    void loadHealth();
    const intervalID = window.setInterval(() => {
      void loadHealth();
    }, 5_000);

    return () => {
      ignore = true;
      window.clearInterval(intervalID);
    };
  }, []);

  return (
    <main className="min-h-screen bg-zinc-950 px-6 py-10 text-zinc-100">
      <section className="mx-auto flex min-h-[calc(100vh-5rem)] max-w-3xl flex-col justify-center">
        <p className="text-sm font-medium uppercase text-cyan-300">Dashboard</p>
        <h1 className="mt-4 text-4xl font-semibold text-white sm:text-5xl">Health check</h1>
        <div className="mt-8 border-l-2 border-cyan-300 pl-5">
          {healthState.status === "loading" ? (
            <p className="text-lg text-zinc-300">Loading live server status...</p>
          ) : null}

          {healthState.status === "ready" ? (
            <div className="space-y-2">
              <p className="text-lg text-zinc-300">API status</p>
              <p className="font-mono text-2xl text-emerald-300">{healthState.health.status}</p>
              <p className="text-sm text-zinc-400">version {healthState.health.version}</p>
            </div>
          ) : null}

          {healthState.status === "error" ? (
            <div className="space-y-2">
              <p className="text-lg text-red-300">API unavailable</p>
              <p className="font-mono text-sm text-zinc-400">{healthState.message}</p>
            </div>
          ) : null}
        </div>
      </section>
    </main>
  );
}
