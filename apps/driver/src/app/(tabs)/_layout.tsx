/** Driver tab shell (Home · Earnings · History · Account) with the floating pill tab bar. Route files only. */
import { PillTabBar } from "@platform/ui";
import { Tabs } from "expo-router";

export default function TabsLayout() {
  return (
    <Tabs tabBar={(props) => <PillTabBar {...props} />} screenOptions={{ headerShown: false }}>
      <Tabs.Screen name="index" options={{ title: "Home", tabBarIconName: "home-variant" } as object} />
      <Tabs.Screen
        name="earnings"
        options={{ title: "Earnings", tabBarIconName: "wallet-outline" } as object}
      />
      <Tabs.Screen
        name="history"
        options={{ title: "History", tabBarIconName: "clipboard-text-clock-outline" } as object}
      />
      <Tabs.Screen
        name="account"
        options={{ title: "Account", tabBarIconName: "account-circle" } as object}
      />
    </Tabs>
  );
}
