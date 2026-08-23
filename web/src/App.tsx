import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { FileText, Home, NotebookPen, ScrollText, Sparkles, Target } from "lucide-react";
import type { ReactNode } from "react";
import { Link, Route, Switch, useLocation } from "wouter";

import { GoalsPage } from "./pages/GoalsPage";
import { LogPage } from "./pages/LogPage";
import { PlanPage } from "./pages/PlanPage";
import { ReviewPage } from "./pages/ReviewPage";
import { SkillsPage } from "./pages/SkillsPage";
import { TodayPage } from "./pages/TodayPage";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 10_000,
    },
  },
});

const navigation = [
  { href: "/", label: "Today", icon: Home },
  { href: "/goals", label: "Goals", icon: Target },
  { href: "/log", label: "Log", icon: ScrollText },
  { href: "/review", label: "Review", icon: NotebookPen },
  { href: "/skills", label: "Skills", icon: Sparkles },
];

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppShell>
        <Switch>
          <Route path="/" component={TodayPage} />
          <Route path="/goals" component={GoalsPage} />
          <Route path="/log" component={LogPage} />
          <Route path="/review" component={ReviewPage} />
          <Route path="/skills" component={SkillsPage} />
          <Route path="/plan" component={PlanPage} />
        </Switch>
      </AppShell>
    </QueryClientProvider>
  );
}

function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-[var(--color-app-bg)] text-[var(--color-app-text)]">
      <div className="mx-auto flex min-h-screen w-full max-w-7xl md:px-6">
        <aside className="hidden w-24 shrink-0 border-r border-white/10 py-6 md:flex md:flex-col md:items-center md:gap-4">
          <div className="grid size-11 place-items-center rounded-md bg-cyan-300 text-sm font-bold text-zinc-950">
            D
          </div>
          <DesktopNav />
          <DesktopPlanLink />
        </aside>

        <main className="min-w-0 flex-1 pb-32 md:pb-8">{children}</main>
      </div>

      <MobileNav />
    </div>
  );
}

function DesktopPlanLink() {
  const [location] = useLocation();
  const active = location === "/plan";

  return (
    <Link
      href="/plan"
      className={[
        "mt-auto grid size-12 place-items-center rounded-md border text-zinc-400 transition hover:text-white",
        active ? "border-cyan-300 bg-cyan-300 text-zinc-950" : "border-transparent",
      ].join(" ")}
      aria-label="Plan"
      title="Plan"
    >
      <FileText className="size-5" aria-hidden="true" />
    </Link>
  );
}

function DesktopNav() {
  const [location] = useLocation();

  return (
    <nav className="mt-4 flex flex-col gap-2" aria-label="Primary">
      {navigation.map((item) => {
        const Icon = item.icon;
        const active = location === item.href;

        return (
          <Link
            key={item.href}
            href={item.href}
            className={[
              "grid size-12 place-items-center rounded-md border text-zinc-400 transition hover:text-white",
              active ? "border-cyan-300 bg-cyan-300 text-zinc-950" : "border-transparent",
            ].join(" ")}
            aria-label={item.label}
            title={item.label}
          >
            <Icon className="size-5" aria-hidden="true" />
          </Link>
        );
      })}
    </nav>
  );
}

function MobileNav() {
  const [location] = useLocation();

  return (
    <nav
      className="fixed inset-x-0 bottom-0 z-30 border-t border-white/10 bg-zinc-950/95 px-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] pt-2 backdrop-blur md:hidden"
      aria-label="Primary"
    >
      <div className="mx-auto grid max-w-md grid-cols-5 gap-1">
        {navigation.map((item) => {
          const Icon = item.icon;
          const active = location === item.href;

          return (
            <Link
              key={item.href}
              href={item.href}
              className={[
                "flex min-h-11 flex-col items-center justify-center gap-1 rounded-md text-xs font-medium",
                active ? "bg-cyan-300 text-zinc-950" : "text-zinc-400",
              ].join(" ")}
            >
              <Icon className="size-5" aria-hidden="true" />
              <span>{item.label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
