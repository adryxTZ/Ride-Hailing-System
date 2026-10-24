/**
 * Privacy request use cases over the app's API client. Pure data access: errors
 * become ApiError with the server's code and localised message (PRIVACY_REQUEST_OPEN,
 * PRIVACY_RATE_LIMITED, EXPORT_NOT_READY, EXPORT_EXPIRED, PRIVACY_REQUEST_NOT_OPEN).
 */
import type { createApiClient } from "@platform/api-client";
import { toApiError } from "@platform/auth";
import type { PrivacyKind, PrivacyRequest } from "./model";

type Client = ReturnType<typeof createApiClient>;

/** A short-lived link to the prepared export file. */
export interface ExportLink {
  url: string;
  expires_at: string;
}

export function createPrivacyService(client: Client) {
  return {
    /** The signed-in person's requests. */
    async list(signal?: AbortSignal): Promise<PrivacyRequest[]> {
      // No error responses are declared for this route, so read the result whole
      // (destructuring narrows `response` to never after the error check).
      const res = await client.GET("/me/privacy-requests", { signal });
      if (!res.data) throw toApiError(res.error, res.response.status);
      return res.data.data;
    },

    /** Asks for a copy of the data (EXPORT) or for the account to be deleted (DELETE). */
    async create(kind: PrivacyKind): Promise<PrivacyRequest> {
      const { data, error, response } = await client.POST("/me/privacy-requests", { body: { kind } });
      if (error || !data) throw toApiError(error, response.status);
      return data;
    },

    /** Withdraws a request staff have not picked up yet. */
    async cancel(id: string): Promise<PrivacyRequest> {
      const { data, error, response } = await client.POST("/me/privacy-requests/{id}/cancel", {
        params: { path: { id } },
      });
      if (error || !data) throw toApiError(error, response.status);
      return data;
    },

    /** A fresh download link for a READY export. */
    async downloadLink(id: string): Promise<ExportLink> {
      const { data, error, response } = await client.GET("/me/privacy-requests/{id}/download", {
        params: { path: { id } },
      });
      if (error || !data) throw toApiError(error, response.status);
      return data;
    },
  };
}

export type PrivacyService = ReturnType<typeof createPrivacyService>;
