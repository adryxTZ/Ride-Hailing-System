/** privacy service — data-protection requests (download my data, delete my account) via the shared client. */
import { createPrivacyService } from "@platform/privacy";
import { api } from "@/services/api";

export const privacy = createPrivacyService(api);
