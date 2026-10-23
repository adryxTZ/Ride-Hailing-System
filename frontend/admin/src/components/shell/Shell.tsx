"use client";
/** Shell — signed-in frame: side navigation, the slim top bar and the page. Signed-out visitors go to /login. */
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { getAccess } from "@/lib/session";
import { Sidebar } from "./Sidebar";
import { TopBar } from "./TopBar";

export function Shell({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [ready, setReady] = useState(false);
  useEffect(() => {
    const check = () => {
      if (!getAccess()) router.replace(`/login?next=${encodeURIComponent(pathname)}`);
      else setReady(true);
    };
    check();
    window.addEventListener("platform-session", check);
    return () => window.removeEventListener("platform-session", check);
  }, [router, pathname]);
  if (!ready) return null;
  return (
    <div className="flex min-h-dvh">
      <a href="#main" className="skip-link">
        Skip to content
      </a>
      <Sidebar />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar />
        <main id="main" className="min-w-0 flex-1 px-4 pb-10 pt-2 sm:px-6">
          {children}
        </main>
      </div>
    </div>
  );
}
