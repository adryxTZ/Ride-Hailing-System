/**
 * EarningsScreen — what the driver earned today and this week, the wallet,
 * paying debt by mobile money, and cashing out what the platform owes.
 */
import { Banner, Button, Header, Screen, Text, useTheme, withAlpha } from "@platform/ui";
import { useLocalSearchParams } from "expo-router";
import { useEffect, useState } from "react";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { CashOutCard } from "@/components/earnings/CashOutCard";
import { CashOutSheet } from "@/components/earnings/CashOutSheet";
import { HoldBanner } from "@/components/earnings/HoldBanner";
import { PayNowSheet } from "@/components/earnings/PayNowSheet";
import { WalletCard } from "@/components/earnings/WalletCard";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useTripHistory, useWallet } from "@/hooks/useDriverTrips";
import { usePayin } from "@/hooks/usePayin";
import { usePayoutActions } from "@/hooks/usePayouts";
import { useProfile } from "@/hooks/useProfile";
import { earningsSince, money, startOfToday, startOfWeek } from "@/models/earnings";
import { cashOutState } from "@/models/payout";
import { mainHold, successText } from "@/models/wallet";

export function EarningsScreen() {
  const { palette } = useTheme();
  const insets = useSafeAreaInsets();
  const { pay } = useLocalSearchParams<{ pay?: string }>();
  const { data: trips } = useTripHistory();
  const wallet = useWallet();
  const { data: me } = useProfile();
  const brandCfg = useAppConfig().data?.brand;
  const payin = usePayin(wallet.data?.pending_payin);
  const [paying, setPaying] = useState(false);
  const [cashingOut, setCashingOut] = useState(false);
  const payout = usePayoutActions();
  const cashOut = wallet.data ? cashOutState(wallet.data) : undefined;
  const today = earningsSince(trips, startOfToday());
  const week = earningsSince(trips, startOfWeek());
  const ink = palette.on_primary ?? palette.background;
  const hold = mainHold(wallet.data?.holds);
  const debtHold = hold?.kind === "DEBT";
  // Arriving from the Home hold panel with ?pay=1, or with a prompt already waiting, opens the sheet.
  useEffect(() => {
    if (pay === "1" || payin.phase === "pending") setPaying(true);
  }, [pay, payin.phase]);
  const closeSheet = () => {
    setPaying(false);
    if (payin.phase !== "pending") payin.reset(); // a waiting prompt keeps waiting
  };
  return (
    <View style={styles.root}>
      <Screen align="top">
        <Header title="Earnings" large />
        {hold && wallet.data ? (
          <HoldBanner
            hold={hold}
            wallet={wallet.data}
            brand={brandCfg?.name}
            supportPhone={brandCfg?.support_phone}
            onPay={() => setPaying(true)}
          />
        ) : null}
        <Animated.View
          entering={FadeInDown.delay(60).duration(280)}
          style={[styles.hero, { backgroundColor: palette.primary }]}
        >
          <Text style={[styles.label, { color: withAlpha(ink, 0.8) }]}>You earned today</Text>
          <Text style={[styles.big, { color: ink }]}>{money(today.total, today.currency)}</Text>
          <Text style={{ color: withAlpha(ink, 0.8) }}>
            {`${today.trips} completed trip${today.trips === 1 ? "" : "s"}`}
          </Text>
        </Animated.View>
        <Animated.View
          entering={FadeInDown.delay(120).duration(280)}
          style={[styles.card, { backgroundColor: palette.surface, shadowColor: palette.text }]}
        >
          <View style={styles.row}>
            <Text variant="muted">You earned this week</Text>
            <Text style={styles.value}>{money(week.total, week.currency)}</Text>
          </View>
          <View style={styles.row}>
            <Text variant="muted">Fares collected this week</Text>
            <Text style={styles.value}>{money(week.fares, week.currency)}</Text>
          </View>
          <View style={styles.row}>
            <Text variant="muted">Platform fee this week</Text>
            <Text style={styles.value}>{money(week.commission, week.currency)}</Text>
          </View>
          <View style={styles.row}>
            <Text variant="muted">Trips this week</Text>
            <Text style={styles.value}>{String(week.trips)}</Text>
          </View>
        </Animated.View>
        <Animated.View entering={FadeInDown.delay(180).duration(280)}>
          {wallet.data ? (
            <WalletCard wallet={wallet.data} onPay={debtHold ? undefined : () => setPaying(true)} />
          ) : wallet.isError ? (
            <View style={styles.retry}>
              <Banner icon="alert-circle-outline" title="Could not load your wallet" />
              <Button title="Retry" variant="secondary" onPress={() => void wallet.refetch()} />
            </View>
          ) : (
            <View style={[styles.card, styles.loading, { backgroundColor: palette.surface }]}>
              <ActivityIndicator color={palette.primary} />
            </View>
          )}
        </Animated.View>
        {cashOut && wallet.data ? (
          <Animated.View entering={FadeInDown.delay(240).duration(280)}>
            <CashOutCard
              state={cashOut}
              currency={wallet.data.currency}
              busy={payout.cancel.isPending}
              onCashOut={() => {
                payout.request.reset();
                setCashingOut(true);
              }}
              onCancel={(id) => payout.cancel.mutate(id)}
            />
          </Animated.View>
        ) : null}
        <Text variant="muted">
          Earnings are fares minus the platform fee. What you owe from cash trips can be paid here by mobile
          money.
        </Text>
      </Screen>
      {cashingOut && cashOut?.kind === "ready" && wallet.data ? (
        <CashOutSheet
          bottom={insets.bottom}
          available={cashOut.available}
          min={cashOut.min}
          currency={wallet.data.currency}
          phone={me?.phone}
          busy={payout.request.isPending}
          error={payout.request.error?.message}
          onSubmit={(amount) => payout.request.mutate(amount, { onSuccess: () => setCashingOut(false) })}
          onClose={() => setCashingOut(false)}
        />
      ) : null}
      {paying && wallet.data ? (
        <PayNowSheet
          bottom={insets.bottom}
          wallet={wallet.data}
          phase={payin.phase}
          error={payin.error}
          phone={me?.phone}
          pendingAmount={payin.payin ? Number(payin.payin.amount) : undefined}
          successText={successText(wallet.data)}
          onPay={(amount, phone) => void payin.start(amount, phone)}
          onRetry={payin.reset}
          onClose={closeSheet}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  hero: { borderRadius: 18, padding: 20, gap: 6 },
  label: { fontSize: 14 },
  big: { fontSize: 34, fontWeight: "800" },
  card: {
    borderRadius: 18,
    padding: 16,
    gap: 12,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  loading: { alignItems: "center", minHeight: 96, justifyContent: "center" },
  retry: { gap: 10 },
  row: { flexDirection: "row", justifyContent: "space-between" },
  value: { fontSize: 16, fontWeight: "700" },
});
