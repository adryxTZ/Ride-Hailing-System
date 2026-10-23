/**
 * ProfileFormScreen — the driver's name, licence number, class and expiry, and
 * national ID. The country's rules (licence format, classes) come from the server.
 */
import { Button, Header, Screen, Text, TextField } from "@platform/ui";
import { useState } from "react";
import { LicenceClassPicker } from "@/components/onboarding/LicenceClassPicker";
import { useBack } from "@/hooks/useBack";
import { useOnboarding, useSaveProfile } from "@/hooks/useOnboarding";

const DATE_HINT = "YYYY-MM-DD";
const datePattern = /^\d{4}-\d{2}-\d{2}$/;

export function ProfileFormScreen() {
  const back = useBack();
  const { data: ob } = useOnboarding();
  const save = useSaveProfile();
  const p = ob?.profile;
  const [name, setName] = useState(p?.full_name ?? "");
  const [licence, setLicence] = useState(p?.licence_number ?? "");
  const [expiry, setExpiry] = useState(p?.licence_expiry ?? "");
  const [nid, setNid] = useState(p?.national_id ?? "");
  const [cls, setCls] = useState(p?.licence_class ?? "");
  const classes = ob?.licence_classes ?? [];
  const licenceBad = licenceFails(licence.trim(), ob?.licence_pattern);
  const expiryBad = expiry !== "" && !datePattern.test(expiry);
  const ready =
    name.trim().length >= 3 &&
    licence.trim() !== "" &&
    !licenceBad &&
    (classes.length === 0 || cls !== "") &&
    nid.trim() !== "" &&
    !expiryBad;

  const submit = () =>
    save.mutate(
      {
        full_name: name,
        licence_number: licence.trim(),
        licence_class: cls || undefined,
        licence_expiry: expiry || undefined,
        national_id: nid,
      },
      { onSuccess: () => back() },
    );

  return (
    <Screen align="top">
      <Header title="Personal details" leading="back" onLeading={() => back()} />
      <Text variant="muted">As written on your driving licence.</Text>
      <TextField
        label="Full name"
        value={name}
        onChangeText={setName}
        autoCapitalize="words"
        textContentType="name"
      />
      <TextField
        label="Driving licence number"
        value={licence}
        onChangeText={setLicence}
        autoCapitalize="characters"
        error={licenceBad ? "Check the number: type it exactly as printed on your licence." : undefined}
      />
      {classes.length ? <LicenceClassPicker classes={classes} value={cls} onChange={setCls} /> : null}
      <TextField
        label="Licence expiry date (optional)"
        value={expiry}
        onChangeText={setExpiry}
        placeholder={DATE_HINT}
        keyboardType="numbers-and-punctuation"
        error={expiryBad ? `Use ${DATE_HINT}` : undefined}
      />
      <TextField
        label="National ID number (NIDA)"
        value={nid}
        onChangeText={setNid}
        keyboardType="number-pad"
      />
      {save.error ? <Text variant="error">{save.error.message}</Text> : null}
      <Button title="Save" onPress={submit} loading={save.isPending} disabled={!ready} />
    </Screen>
  );
}

/** True when a typed licence number breaks the country's pattern (nothing typed is not an error yet). */
function licenceFails(value: string, pattern?: string): boolean {
  if (!value || !pattern) return false;
  try {
    return !new RegExp(pattern).test(value);
  } catch {
    return false; // a pattern the app cannot read is checked by the server
  }
}
