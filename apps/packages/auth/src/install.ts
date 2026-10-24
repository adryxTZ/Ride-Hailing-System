import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

const KEY = "platform.install.v1";
const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

let cached: string | null = null;

/**
 * A random v4 UUID, made once per app install and kept in the secure store.
 * Sent as X-Install-Id when signing in so the server can notice one install
 * used by both a rider and a driver account (the self-booking check). It is
 * an identifier, not a secret, so Math.random is enough and no native crypto
 * module is needed.
 */
function newUuidV4(): string {
  const hex = Array.from({ length: 32 }, () => Math.floor(Math.random() * 16).toString(16));
  hex[12] = "4";
  hex[16] = "89ab"[Math.floor(Math.random() * 4)];
  const s = hex.join("");
  return `${s.slice(0, 8)}-${s.slice(8, 12)}-${s.slice(12, 16)}-${s.slice(16, 20)}-${s.slice(20)}`;
}

async function read(): Promise<string | null> {
  if (Platform.OS === "web") return globalThis.localStorage?.getItem(KEY) ?? null;
  return SecureStore.getItemAsync(KEY);
}

async function write(value: string): Promise<void> {
  if (Platform.OS === "web") globalThis.localStorage?.setItem(KEY, value);
  else await SecureStore.setItemAsync(KEY, value);
}

/** This install's id (created on first use). */
export async function installId(): Promise<string> {
  if (cached) return cached;
  const stored = await read().catch(() => null);
  if (stored && UUID_V4.test(stored)) {
    cached = stored;
    return stored;
  }
  const id = newUuidV4();
  cached = id;
  await write(id).catch(() => undefined);
  return id;
}
