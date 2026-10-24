import type { createApiClient } from "@platform/api-client";
import { useConsentStore } from "./consentStore";
import { type AppKind, type Session, sessionFromTokens, useSessionStore } from "./session";

type Client = ReturnType<typeof createApiClient>;

/** Error carrying the server's stable code so screens can map it to copy. */
export class ApiError extends Error {
  constructor(
    public code: string,
    message: string,
    public status: number,
  ) {
    super(message);
  }
}

/** Maps an openapi-fetch error body to ApiError (code + server-localised message). */
export function toApiError(error: unknown, status: number): ApiError {
  const e = (error as { error?: { code?: string; message?: string } } | undefined)?.error;
  return new ApiError(e?.code ?? "UNKNOWN", e?.message ?? "Request failed", status);
}

/** Use cases for signing in and out. Pure functions over the client and the session store. */
export function createAuthService(client: Client, app: AppKind) {
  return {
    /** Sends the OTP. Returns the dev code when the server echoes it (development only). */
    async requestOtp(phone: string): Promise<{ devCode?: string }> {
      const { data, error, response } = await client.POST("/auth/otp/request", { body: { phone, app } });
      if (error) throw toApiError(error, response.status);
      return { devCode: data?.dev_code };
    },

    /** Verifies the code, stores the resulting session and notes whether consent is needed. */
    async verifyOtp(phone: string, code: string): Promise<Session> {
      const { data, error, response } = await client.POST("/auth/otp/verify", { body: { phone, code, app } });
      if (error || !data) throw toApiError(error, response.status);
      noteConsent(data.consent_required);
      const session = sessionFromTokens(data.access_token, data.refresh_token);
      await useSessionStore.getState().set(session);
      return session;
    },

    /** Rotates the refresh token. On failure the session is cleared (user must sign in again). */
    async refresh(): Promise<Session | null> {
      const current = useSessionStore.getState().session;
      if (!current) return null;
      const { data, error, response } = await client.POST("/auth/refresh", {
        body: { refresh_token: current.refreshToken, app },
      });
      if (error || !data) {
        if (response.status === 401) await useSessionStore.getState().clear();
        throw toApiError(error, response.status);
      }
      noteConsent(data.consent_required);
      const session = sessionFromTokens(data.access_token, data.refresh_token);
      await useSessionStore.getState().set(session);
      return session;
    },

    /** Revokes the session family server-side and clears local state. */
    async logout(): Promise<void> {
      const current = useSessionStore.getState().session;
      if (current) {
        await client
          .POST("/auth/logout", { body: { refresh_token: current.refreshToken } })
          .catch(() => undefined);
      }
      useConsentStore.getState().setRequired(false);
      await useSessionStore.getState().clear();
    },
  };
}

/** Copies the token pair's consent flag into the store (absent from older servers: leave as is). */
function noteConsent(required: boolean | undefined): void {
  if (required !== undefined) useConsentStore.getState().setRequired(required);
}

export type AuthService = ReturnType<typeof createAuthService>;
