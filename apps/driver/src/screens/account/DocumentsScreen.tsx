/**
 * DocumentsScreen — an approved driver's documents: each one the country asks
 * for, its expiry (valid, expiring, expired), a renewal under review, and a
 * Renew button that sends a new copy for review while the current one stays valid.
 */
import { Header, Icon, ScalePressable, Screen, Text, useTheme, withAlpha } from "@platform/ui";
import { ActivityIndicator, StyleSheet, View } from "react-native";
import Animated, { FadeInDown } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { ExpiryDateSheet } from "@/components/documents/ExpiryDateSheet";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { useDocumentCapture } from "@/hooks/useDocumentCapture";
import { useDocuments } from "@/hooks/useDocuments";
import { localText } from "@/hooks/useTaxiClasses";
import { type ChipTone, type DriverDocument, expiryChip, type Requirement } from "@/models/document";

const ENTER_STAGGER_MS = 60;

export function DocumentsScreen() {
  const back = useBack();
  const insets = useSafeAreaInsets();
  const locale = useAppConfig().data?.brand.locale ?? "en";
  const { data, isLoading, error } = useDocuments();
  const capture = useDocumentCapture("renew");
  return (
    <Screen align="top">
      <Header title="Documents" leading="back" onLeading={() => back()} />
      <Text variant="muted">
        Keep these valid to keep driving. We remind you 30, 7 and 1 days before one expires.
      </Text>
      {isLoading ? <ActivityIndicator /> : null}
      {error ? <Text variant="error">{error.message}</Text> : null}
      {(data?.requirements ?? []).map((req, i) => {
        const doc = data?.documents.find((d) => d.kind === req.kind);
        const label = localText(req.label, locale);
        return (
          <DocumentRow
            key={req.kind}
            index={i}
            req={req}
            label={label}
            doc={doc}
            busy={capture.busyKind === req.kind}
            onRenew={() =>
              void capture.capture({
                kind: req.kind,
                label,
                tracksExpiry: req.tracks_expiry,
                expiryRequired: req.expiry_required,
              })
            }
          />
        );
      })}
      {capture.error ? <Text variant="error">{capture.error}</Text> : null}
      {capture.asking ? (
        <ExpiryDateSheet
          label={capture.asking.label}
          required={capture.asking.required}
          bottom={insets.bottom}
          onDone={capture.asking.answer}
          onCancel={capture.asking.cancel}
        />
      ) : null}
    </Screen>
  );
}

function DocumentRow({
  req,
  label,
  doc,
  index,
  busy,
  onRenew,
}: {
  req: Requirement;
  label: string;
  doc?: DriverDocument;
  index: number;
  busy: boolean;
  onRenew: () => void;
}) {
  const { palette } = useTheme();
  const chip = expiryChip(doc);
  const tone: Record<ChipTone, string> = {
    ok: palette.success ?? palette.primary,
    warn: palette.warning ?? palette.primary,
    bad: palette.danger ?? palette.primary,
    muted: palette.muted ?? palette.text,
  };
  const status = !doc
    ? req.mandatory
      ? "Not provided"
      : "Optional, not provided"
    : doc.status === "REJECTED"
      ? `Rejected: ${doc.status_reason ?? ""}`
      : doc.status === "PENDING"
        ? "Under review"
        : "Approved";
  return (
    <Animated.View
      entering={FadeInDown.delay(index * ENTER_STAGGER_MS).duration(260)}
      style={[styles.row, { backgroundColor: palette.surface, shadowColor: palette.text }]}
    >
      <View style={styles.body}>
        <Text style={styles.title}>{label}</Text>
        <Text variant="muted">{status}</Text>
        {chip ? (
          <View style={[styles.chip, { backgroundColor: withAlpha(tone[chip.tone], 0.14) }]}>
            <Text style={[styles.chipText, { color: tone[chip.tone] }]}>{chip.text}</Text>
          </View>
        ) : null}
        {doc?.renewal?.status === "REJECTED" ? (
          <Text variant="error">{`Renewal rejected: ${doc.renewal.status_reason ?? ""}`}</Text>
        ) : null}
      </View>
      <ScalePressable
        accessibilityRole="button"
        accessibilityLabel={`Renew ${label}`}
        onPress={onRenew}
        disabled={busy}
        style={[styles.renew, { backgroundColor: withAlpha(palette.primary, 0.12) }]}
      >
        {busy ? (
          <ActivityIndicator color={palette.primary} />
        ) : (
          <Icon name="camera-plus-outline" size={20} color={palette.primary} />
        )}
        <Text style={[styles.renewText, { color: palette.primary }]}>{doc ? "Renew" : "Add"}</Text>
      </ScalePressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    borderRadius: 18,
    padding: 16,
    shadowOpacity: 0.05,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 4 },
    elevation: 2,
  },
  body: { flex: 1, gap: 4 },
  title: { fontSize: 16, fontWeight: "700" },
  chip: {
    alignSelf: "flex-start",
    borderRadius: 999,
    paddingHorizontal: 10,
    paddingVertical: 3,
    marginTop: 2,
  },
  chipText: { fontSize: 13, fontWeight: "600" },
  renew: {
    minHeight: 44,
    borderRadius: 12,
    paddingHorizontal: 12,
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  renewText: { fontWeight: "700" },
});
