/**
 * Consent use cases behind the consent screen: read the current Privacy Notice
 * and accept it. Accepting clears the flag in consentStore.ts.
 */
import type { components, createApiClient } from "@platform/api-client";
import { toApiError } from "./authService";
import { useConsentStore } from "./consentStore";
import { installId } from "./install";
import type { AppKind } from "./session";

type Client = ReturnType<typeof createApiClient>;

export type ConsentDocument = components["schemas"]["ConsentDocument"];
export type ConsentStatus = components["schemas"]["ConsentStatus"];

/** Sent with the acceptance, as at sign-in (the server's fraud checks). */
const INSTALL_HEADER = "X-Install-Id";

/** Use cases behind the consent screen, bound to the app's client. */
export function createConsentService(client: Client, app: AppKind) {
  const consentApp = app === "driver" ? "DRIVER" : "RIDER";
  return {
    /** The current notice for this app (title, summary, version, full text URL). */
    async document(signal?: AbortSignal): Promise<ConsentDocument> {
      const { data, error, response } = await client.GET("/legal/consent", {
        params: { query: { app } },
        signal,
      });
      if (error || !data) throw toApiError(error, response.status);
      return data;
    },

    /**
     * Records acceptance of `version` and clears the flag. Throws ApiError
     * CONSENT_VERSION_STALE when a newer notice was published meanwhile.
     */
    async accept(version: number): Promise<void> {
      const { error, response } = await client.POST("/me/consents", {
        body: { app: consentApp, version },
        params: { header: { [INSTALL_HEADER]: await installId() } },
      });
      if (error) throw toApiError(error, response.status);
      useConsentStore.getState().setRequired(false);
    },
  };
}

export type ConsentService = ReturnType<typeof createConsentService>;

/**
 * Is this an update to a notice the person already accepted (rather than their
 * first time)? The consent screen then says the notice has changed.
 */
export function isConsentUpdate(status?: ConsentStatus | null): boolean {
  const accepted = status?.accepted_version;
  return accepted != null && !!status && accepted < status.current_version;
}
