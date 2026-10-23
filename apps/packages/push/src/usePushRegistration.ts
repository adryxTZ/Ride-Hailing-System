/**
 * usePushRegistration — once signed in, asks for notification permission,
 * creates the Android channels the server addresses ("offers" loud and urgent,
 * "trips", "chat"), gets the Expo push token and hands it to `register`.
 *
 * Needs a development or store build with an EAS project id
 * (expo.extra.eas.projectId). Expo Go on Android ships without the push
 * module, so expo-notifications is only loaded where push can work; elsewhere
 * the hook does nothing.
 */
import Constants from "expo-constants";
import * as Device from "expo-device";
import { useEffect, useRef } from "react";
import { Platform } from "react-native";

type NotificationsModule = typeof import("expo-notifications");

export interface PushDevice {
  app: "RIDER" | "DRIVER";
  platform: "ANDROID" | "IOS" | "WEB";
  push_token: string;
  app_version?: string;
}

interface Options {
  enabled: boolean;
  app: PushDevice["app"];
  appVersion?: string;
  register: (d: PushDevice) => Promise<void>;
  /** Called when the person taps a notification (data as sent by the server). */
  onOpen?: (data: Record<string, unknown>) => void;
}

/** The Expo project id push tokens are issued for, when the build has one. */
function projectId(): string | undefined {
  const extra = Constants.expoConfig?.extra as { eas?: { projectId?: string } } | undefined;
  return extra?.eas?.projectId ?? Constants.easConfig?.projectId;
}

/** True where remote push can work (a real device, not Expo Go on Android, with a project id). */
export function pushAvailable(): boolean {
  const expoGo = Constants.executionEnvironment === "storeClient";
  return Device.isDevice && Platform.OS !== "web" && !(expoGo && Platform.OS === "android") && !!projectId();
}

let loaded: NotificationsModule | undefined;

/** Loads expo-notifications once, and only where it is available. */
function notifications(): NotificationsModule | undefined {
  if (!pushAvailable()) return undefined;
  if (!loaded) {
    loaded = require("expo-notifications") as NotificationsModule;
    // Show notifications even while the app is open (the screen may be elsewhere).
    loaded.setNotificationHandler({
      handleNotification: async () => ({
        shouldShowBanner: true,
        shouldShowList: true,
        shouldPlaySound: true,
        shouldSetBadge: false,
      }),
    });
  }
  return loaded;
}

async function obtainToken(N: NotificationsModule): Promise<string | undefined> {
  if (Platform.OS === "android") {
    const channels: [string, string, number][] = [
      ["offers", "Trip requests", N.AndroidImportance.MAX],
      ["trips", "Trip updates", N.AndroidImportance.HIGH],
      ["chat", "Messages", N.AndroidImportance.HIGH],
    ];
    for (const [id, name, importance] of channels) {
      await N.setNotificationChannelAsync(id, { name, importance, sound: "default" });
    }
  }
  const current = await N.getPermissionsAsync();
  const granted = current.granted || (await N.requestPermissionsAsync()).granted;
  if (!granted) return undefined;
  return (await N.getExpoPushTokenAsync({ projectId: projectId() })).data;
}

export function usePushRegistration(o: Options) {
  const opened = useRef(o.onOpen);
  opened.current = o.onOpen;
  useEffect(() => {
    const N = o.enabled ? notifications() : undefined;
    if (!N) return;
    let cancelled = false;
    obtainToken(N)
      .then((token) => {
        if (!token || cancelled) return;
        const platform = Platform.OS === "ios" ? "IOS" : "ANDROID";
        return o.register({ app: o.app, platform, push_token: token, app_version: o.appVersion });
      })
      .catch(() => undefined); // push is a convenience; never block the app
    const sub = N.addNotificationResponseReceivedListener((r) =>
      opened.current?.(r.notification.request.content.data ?? {}),
    );
    return () => {
      cancelled = true;
      sub.remove();
    };
  }, [o.enabled, o.app, o.appVersion, o.register]);
}
