"use client";
import { motion } from "motion/react";
/** LoginScreen — staff sign in with their email and password (dashboard access only). */
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Field, Input } from "@/components/ui/Field";
import { enter } from "@/components/ui/motion";
import { ErrorNote } from "@/components/ui/States";
import { ApiError, api, unwrap } from "@/lib/api";
import { useBrand } from "@/lib/brand";
import { saveTokens } from "@/lib/session";

export function LoginScreen() {
  return (
    <Suspense>
      <LoginForm />
    </Suspense>
  );
}

function LoginForm() {
  const router = useRouter();
  const next = useSearchParams().get("next") ?? "/";
  const { data: brand } = useBrand();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<Error>();

  const signIn = async () => {
    setBusy(true);
    setError(undefined);
    try {
      const location = await currentLocation();
      saveTokens(unwrap(await api.POST("/auth/staff/login", { body: { email, password, location } })));
      router.replace(next);
    } catch (e) {
      const err = e as ApiError;
      setError(
        err.code === "INVALID_CREDENTIALS"
          ? new Error("Wrong email or password.")
          : err.code === "LOGIN_LOCKED"
            ? new Error("Too many attempts. Try again in 10 minutes.")
            : err,
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="grid min-h-dvh place-items-center p-6">
      <motion.form
        {...enter(0)}
        onSubmit={(e) => {
          e.preventDefault();
          void signIn();
        }}
        className="grid w-full max-w-sm gap-5 rounded-[10px] bg-surface p-8"
      >
        <div className="flex items-center gap-3">
          <span className="grid size-11 place-items-center rounded-full bg-brand text-lg font-bold text-brand-ink">
            {brand?.brand.name.slice(0, 1) ?? ""}
          </span>
          <div>
            <h1 className="text-xl font-semibold">{brand?.brand.name ?? ""} Operations</h1>
            <p className="text-sm text-muted">Sign in with your staff email</p>
          </div>
        </div>
        <Field label="Email">
          <Input
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </Field>
        <Field label="Password">
          <Input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        <ErrorNote error={error} />
        <Button
          type="submit"
          variant="primary"
          loading={busy}
          disabled={!email.includes("@") || password.length < 6}
        >
          Sign in
        </Button>
        <p className="text-center text-xs text-muted">
          No account? Ask a Super Admin to add you under Staff.
        </p>
      </motion.form>
    </main>
  );
}

/** How long to wait for the browser's position before signing in without it. */
const LOCATION_WAIT_MS = 4000;

/** The browser's position, or undefined when it is refused, unavailable or slow. */
function currentLocation(): Promise<{ lat: number; lng: number; accuracy_m: number } | undefined> {
  if (typeof navigator === "undefined" || !navigator.geolocation) return Promise.resolve(undefined);
  return new Promise((resolve) => {
    const done = setTimeout(() => resolve(undefined), LOCATION_WAIT_MS);
    navigator.geolocation.getCurrentPosition(
      (p) => {
        clearTimeout(done);
        resolve({ lat: p.coords.latitude, lng: p.coords.longitude, accuracy_m: p.coords.accuracy });
      },
      () => {
        clearTimeout(done);
        resolve(undefined);
      },
      { enableHighAccuracy: false, timeout: LOCATION_WAIT_MS, maximumAge: 60_000 },
    );
  });
}
