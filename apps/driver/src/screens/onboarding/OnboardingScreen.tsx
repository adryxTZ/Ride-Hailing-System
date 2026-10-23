/**
 * OnboardingScreen — the driver's registration checklist: where it stands,
 * personal details, the car, and the document photos; "Send for review" once
 * everything is there. Read-only while the documents are being checked.
 */
import { Button, Header, Screen, Text } from "@platform/ui";
import { useRouter } from "expo-router";
import { StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { ExpiryDateSheet } from "@/components/documents/ExpiryDateSheet";
import { ChecklistRow } from "@/components/onboarding/ChecklistRow";
import { StatusCard } from "@/components/onboarding/StatusCard";
import { useAppConfig } from "@/hooks/useAppConfig";
import { useBack } from "@/hooks/useBack";
import { useDocumentCapture } from "@/hooks/useDocumentCapture";
import { useOnboarding, useSubmitOnboarding } from "@/hooks/useOnboarding";
import { localText, useTaxiClasses } from "@/hooks/useTaxiClasses";
import { documentFor, documentStatus, progress, statusCopy } from "@/models/onboarding";

const DOC_ICON: Record<string, string> = {
  PHOTO: "account-box-outline",
  LICENCE: "card-account-details-outline",
  NATIONAL_ID: "badge-account-horizontal-outline",
  REGISTRATION: "file-document-outline",
  INSURANCE: "shield-check-outline",
  VEHICLE_PHOTO: "car-side",
  POLICE_CLEARANCE: "shield-account-outline",
  INSPECTION: "clipboard-check-outline",
};

export function OnboardingScreen() {
  const router = useRouter();
  const back = useBack();
  const { data: config } = useAppConfig();
  const locale = config?.brand.locale ?? "en";
  const { data: ob, isLoading, error, refetch } = useOnboarding();
  const submit = useSubmitOnboarding();
  const docs = useDocumentCapture();
  const insets = useSafeAreaInsets();
  const { classes } = useTaxiClasses();

  if (!ob) {
    return (
      <Screen align="top">
        <Header title="Registration" leading="back" onLeading={() => back()} />
        <Text variant="muted">{isLoading ? "Loading…" : (error?.message ?? "")}</Text>
        {error ? <Button title="Try again" onPress={() => void refetch()} /> : null}
      </Screen>
    );
  }
  const copy = statusCopy(ob.profile.status);
  const { done, total } = progress(ob);
  const edit = ob.can_edit;
  const v = ob.vehicle;
  const className = v ? localText(classes.find((c) => c.key === v.vehicle_class)?.name, locale) : "";
  let i = 0;

  return (
    <Screen align="top">
      <Header title="Registration" leading="back" onLeading={() => back()} />
      <StatusCard
        icon={copy.icon}
        title={copy.title}
        body={copy.body}
        reason={ob.profile.status === "REJECTED" ? (ob.profile.status_reason ?? undefined) : undefined}
        done={done}
        total={total}
        showProgress={edit}
      />
      <Text style={styles.section}>You</Text>
      <ChecklistRow
        index={i++}
        icon="account-outline"
        title="Personal details"
        status={
          ob.missing.includes("profile") ? "Name, licence and ID number" : (ob.profile.full_name ?? "Added")
        }
        tone={ob.missing.includes("profile") ? "todo" : "ok"}
        disabled={!edit}
        onPress={() => router.push("/onboarding/profile")}
      />
      <Text style={styles.section}>Your car</Text>
      <ChecklistRow
        index={i++}
        icon="car-outline"
        title="Vehicle"
        status={v ? `${className || v.vehicle_class} · ${v.plate}` : "Type, plate and model"}
        tone={v ? (v.status === "REJECTED" ? "bad" : "ok") : "todo"}
        disabled={!edit}
        onPress={() => router.push("/onboarding/vehicle")}
      />
      <Text style={styles.section}>Photos</Text>
      {ob.requirements.map((req) => {
        const doc = documentFor(ob, req);
        const st = documentStatus(doc);
        const label = localText(req.label, locale);
        const needsVehicle = req.owner === "VEHICLE" && !v;
        return (
          <ChecklistRow
            key={req.kind}
            index={i++}
            icon={DOC_ICON[req.kind] ?? "file-outline"}
            title={req.mandatory ? label : `${label} (optional)`}
            status={needsVehicle ? "Add your vehicle first" : st.label}
            tone={st.tone}
            busy={docs.busyKind === req.kind}
            disabled={!edit || needsVehicle}
            onPress={() =>
              void docs.capture({
                kind: req.kind,
                label,
                tracksExpiry: req.tracks_expiry,
                expiryRequired: req.expiry_required,
              })
            }
          />
        );
      })}
      {docs.error ? <Text variant="error">{docs.error}</Text> : null}
      {docs.asking ? (
        <ExpiryDateSheet
          label={docs.asking.label}
          required={docs.asking.required}
          bottom={insets.bottom}
          onDone={docs.asking.answer}
          onCancel={docs.asking.cancel}
        />
      ) : null}
      {submit.error ? <Text variant="error">{submit.error.message}</Text> : null}
      {edit ? (
        <View style={styles.submit}>
          <Button
            title={
              ob.can_submit ? "Send for review" : `${total - done} step${total - done === 1 ? "" : "s"} left`
            }
            onPress={() => submit.mutate(undefined, { onSuccess: () => back() })}
            loading={submit.isPending}
            disabled={!ob.can_submit}
          />
        </View>
      ) : null}
    </Screen>
  );
}

const styles = StyleSheet.create({
  section: { fontSize: 18, fontWeight: "700", marginTop: 4 },
  submit: { marginTop: 8 },
});
