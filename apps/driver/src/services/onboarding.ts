/**
 * onboarding service — the driver's sign-up checklist: profile, vehicle,
 * document uploads (signed URL, then attach) and submit. Pure data access.
 */
import type { components } from "@platform/api-client";
import { API_BASE_URL } from "@/config";
import { api } from "@/services/api";

export type Onboarding = components["schemas"]["Onboarding"];
export type DriverProfileInput = components["schemas"]["DriverProfileInput"];
export type VehicleInput = components["schemas"]["VehicleInput"];
export type Requirement = components["schemas"]["Requirement"];
export type DriverDocument = components["schemas"]["Document"];

/** An API failure that keeps the server's error code and field details. */
export class OnboardingError extends Error {
  constructor(
    message: string,
    readonly code?: string,
  ) {
    super(message);
  }
}

const failure = (error: unknown, fallback: string) => {
  const e = (error as { error?: { message?: string; code?: string } } | undefined)?.error;
  return new OnboardingError(e?.message ?? fallback, e?.code);
};

export async function fetchOnboarding(signal?: AbortSignal): Promise<Onboarding> {
  const { data, error } = await api.GET("/driver/onboarding", { signal });
  if (error || !data) throw failure(error, "Could not load your registration");
  return data;
}

export async function saveProfile(body: DriverProfileInput): Promise<Onboarding> {
  const { data, error } = await api.PUT("/driver/onboarding/profile", { body });
  if (error || !data) throw failure(error, "Could not save your details");
  return data;
}

export async function saveVehicle(body: VehicleInput): Promise<Onboarding> {
  const { data, error } = await api.PUT("/driver/onboarding/vehicle", { body });
  if (error || !data) throw failure(error, "Could not save the vehicle");
  return data;
}

export async function submitOnboarding(): Promise<Onboarding> {
  const { data, error } = await api.POST("/driver/onboarding/submit");
  if (error || !data) throw failure(error, "Could not submit");
  return data;
}

/** The API origin ("http://host:8080") for links the server returns relative ("/v1/files/…"). */
const API_ORIGIN = API_BASE_URL.replace(/\/v1\/?$/, "");
const absolute = (url: string) => (url.startsWith("/") ? API_ORIGIN + url : url);

type UploadType = "image/jpeg" | "image/png" | "image/webp" | "application/pdf";

/** Uploads a photo or PDF from the phone and attaches it as a document of `kind`. */
export async function uploadDocument(
  kind: string,
  fileUri: string,
  contentType: UploadType,
  expiresOn?: string,
): Promise<Onboarding> {
  const fileKey = await uploadFile(fileUri, contentType);
  const { data, error } = await api.POST("/driver/onboarding/documents", {
    body: { kind, file_key: fileKey, content_type: contentType, expires_on: expiresOn },
  });
  if (error || !data) throw failure(error, "Could not attach the document");
  return data;
}

/** Uploads a renewed document for an approved driver; it waits for review beside the current one. */
export async function renewDocument(
  kind: string,
  fileUri: string,
  contentType: UploadType,
  expiresOn?: string,
) {
  const fileKey = await uploadFile(fileUri, contentType);
  const { data, error } = await api.POST("/driver/documents/renewals", {
    body: { kind, file_key: fileKey, content_type: contentType, expires_on: expiresOn },
  });
  if (error || !data) throw failure(error, "Could not send the renewal");
  return data;
}

/** The driver's documents with expiry states. */
export async function fetchDocuments(signal?: AbortSignal) {
  const { data, error } = await api.GET("/driver/documents", { signal });
  if (error || !data) throw failure(error, "Could not load your documents");
  return data;
}

/** PUTs the file to a signed upload address and returns its key. */
async function uploadFile(fileUri: string, contentType: UploadType): Promise<string> {
  const ticket = await api.POST("/driver/onboarding/uploads", { body: { content_type: contentType } });
  if (ticket.error || !ticket.data) throw failure(ticket.error, "Could not start the upload");
  const file = await (await fetch(fileUri)).blob();
  const put = await fetch(absolute(ticket.data.upload_url), {
    method: "PUT",
    headers: { "Content-Type": contentType },
    body: file,
  });
  if (!put.ok) throw new OnboardingError("The upload failed. Please try again.");
  return ticket.data.file_key;
}

export type DriverAreas = components["schemas"]["AreaRef"][];
export type AreaChoices = { country: string; areas: DriverAreas; selected: DriverAreas; primary: string };

/** The regions the driver can work in, and where they are registered now. */
export async function fetchDriverAreas(signal?: AbortSignal): Promise<AreaChoices> {
  const { data, error } = await api.GET("/driver/areas", { signal });
  if (error || !data) throw failure(error, "Could not load the regions");
  return data;
}
