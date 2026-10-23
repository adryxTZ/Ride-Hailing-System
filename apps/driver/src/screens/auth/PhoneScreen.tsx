/** PhoneScreen — step 1 of sign-in: enter a phone number, request the OTP. */
import { ApiError } from "@platform/auth";
import { Button, Screen, Text, TextField } from "@platform/ui";
import { useRouter } from "expo-router";
import { useState } from "react";
import { API_BASE_URL } from "@/config";
import { useAppConfig } from "@/hooks/useAppConfig";
import { auth } from "@/services/api";

export function PhoneScreen() {
  const router = useRouter();
  const { data } = useAppConfig();
  const [phone, setPhone] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const [step, setStep] = useState("idle");

  const submit = async () => {
    setBusy(true);
    setError(undefined);
    try {
      setStep("sending");
      const { devCode } = await auth.requestOtp(phone.trim());
      setStep(`code sent${devCode ? ` (dev code ${devCode})` : ""}, opening code screen`);
      router.push({
        pathname: "/(auth)/otp",
        params: devCode ? { phone: phone.trim(), devCode } : { phone: phone.trim() },
      });
    } catch (e) {
      // Surface the real message: the stable code when the server answered, otherwise the raw error.
      const detail =
        e instanceof ApiError ? `${e.message} (${e.code})` : e instanceof Error ? e.message : String(e);
      console.error("requestOtp failed", e);
      setError(detail);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <Text variant="title">{data?.brand.name ?? ""}</Text>
      <Text variant="muted">Enter your phone number to sign in</Text>
      <TextField
        label="Phone number"
        placeholder="07XX XXX XXX"
        keyboardType="phone-pad"
        autoComplete="tel"
        value={phone}
        onChangeText={setPhone}
        error={error}
        returnKeyType="send"
        onSubmitEditing={() => phone.trim().length >= 9 && void submit()}
      />
      <Button title="Send code" onPress={submit} loading={busy} disabled={phone.trim().length < 9} />
      {__DEV__ ? (
        <Text variant="muted">
          dev · api {API_BASE_URL} · {step}
        </Text>
      ) : null}
    </Screen>
  );
}
