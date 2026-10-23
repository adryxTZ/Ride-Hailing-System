import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";
import { create } from "zustand";

/** Which app this session belongs to; sent on every auth call. */
export type AppKind = "rider" | "driver";

export interface Session {
  accessToken: string;
  refreshToken: string;
  /** Unix seconds when the access token expires (from the JWT `exp`). */
  expiresAt: number;
  userId: string;
  roles: string[];
}

const KEY = "platform.session.v1";

/**
 * Persistence adapter: the device secure store on iOS/Android; localStorage on
 * the web build (development and smoke tests only — the web target is not a
 * shipped surface).
 */
const storage = {
  async get(): Promise<string | null> {
    if (Platform.OS === "web") return globalThis.localStorage?.getItem(KEY) ?? null;
    return SecureStore.getItemAsync(KEY);
  },
  async set(value: string): Promise<void> {
    if (Platform.OS === "web") {
      globalThis.localStorage?.setItem(KEY, value);
      return;
    }
    await SecureStore.setItemAsync(KEY, value);
  },
  async remove(): Promise<void> {
    if (Platform.OS === "web") {
      globalThis.localStorage?.removeItem(KEY);
      return;
    }
    await SecureStore.deleteItemAsync(KEY);
  },
};

interface SessionState {
  /** undefined = not loaded yet; null = signed out. */
  session: Session | null | undefined;
  hydrate: () => Promise<void>;
  set: (s: Session) => Promise<void>;
  clear: () => Promise<void>;
}

/** Session store: memory for speed, secure store for persistence. */
export const useSessionStore = create<SessionState>((set) => ({
  session: undefined,
  async hydrate() {
    try {
      const raw = await storage.get();
      set({ session: raw ? (JSON.parse(raw) as Session) : null });
    } catch {
      set({ session: null });
    }
  },
  async set(s) {
    set({ session: s });
    await storage.set(JSON.stringify(s));
  },
  async clear() {
    set({ session: null });
    await storage.remove();
  },
}));

/** Reads `sub`, `exp` and `roles` from the access token without verifying it (the server verifies). */
export function sessionFromTokens(accessToken: string, refreshToken: string): Session {
  const payload = decodeJwtPayload(accessToken);
  return {
    accessToken,
    refreshToken,
    expiresAt: typeof payload.exp === "number" ? payload.exp : 0,
    userId: typeof payload.sub === "string" ? payload.sub : "",
    roles: Array.isArray(payload.roles) ? (payload.roles as string[]) : [],
  };
}

function decodeJwtPayload(token: string): Record<string, unknown> {
  try {
    const part = token.split(".")[1] ?? "";
    const base64 = part.replace(/-/g, "+").replace(/_/g, "/");
    const json = globalThis.atob(base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), "="));
    return JSON.parse(json) as Record<string, unknown>;
  } catch {
    return {};
  }
}
