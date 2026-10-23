/** usePush — registers this phone for push once signed in and opens the right screen on tap. */
import { useSession } from "@platform/auth";
import { usePushRegistration } from "@platform/push";
import { router } from "expo-router";
import { APP_VERSION } from "@/config";
import { registerDevice } from "@/services/messaging";

function open(data: Record<string, unknown>) {
  if (data.type === "chat" && typeof data.trip_id === "string")
    router.push({ pathname: "/chat/[id]", params: { id: data.trip_id } });
  else router.navigate("/"); // offers and trip updates live on the home screen
}

export function usePush() {
  const { isSignedIn } = useSession();
  usePushRegistration({
    enabled: isSignedIn,
    app: "DRIVER",
    appVersion: APP_VERSION,
    register: registerDevice,
    onOpen: open,
  });
}
