/** realtime service — where the socket lives and a fresh token for each connection attempt. */
import { API_BASE_URL } from "@/config";
import { freshAccessToken } from "@/services/api";

export const REALTIME_URL = `${API_BASE_URL.replace(/^http/, "ws")}/ws`;

/** Shares the API client's single-flight refresh (see @platform/auth freshAccessToken). */
export const freshToken = freshAccessToken;
