/** OtpScreen — step 2 of sign-in: enter the 6-digit code. Shows the dev code when the server echoes it. */
import { ApiError } from "@platform/auth";
import { Button, Screen, Text, TextField } from "@platform/ui";
import { useRouter } from "expo-router";
import { useState } from "react";
import { auth } from "@/services/api";

const CODE_LENGTH = 6;
const MIN_CODE_LENGTH = 4; // test numbers use a short fixed code

export function OtpScreen({ phone, devCode }: { phone: string; devCode?: string }) {
  const router = useRouter();
  const [code, setCode] = useState(devCode ?? "");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const submit = async () => {
    setBusy(true);
    setError(undefined);
    try {
      await auth.verifyOtp(phone, code.trim());
      router.replace("/");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Could not verify the code.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Screen>
      <Text variant="title">Enter the code</Text>
      <Text variant="muted">We sent a code to {phone}</Text>
      {devCode ? <Text variant="muted">Development: code is {devCode}</Text> : null}
      <TextField
        label="Code"
        keyboardType="number-pad"
        maxLength={CODE_LENGTH}
        value={code}
        onChangeText={setCode}
        error={error}
        autoFocus
        returnKeyType="done"
        onSubmitEditing={() => code.trim().length >= MIN_CODE_LENGTH && void submit()}
      />
      <Button
        title="Verify"
        onPress={submit}
        loading={busy}
        disabled={code.trim().length < MIN_CODE_LENGTH}
      />
      <Button
        title="Use a different number"
        variant="secondary"
        onPress={() => (router.canGoBack() ? router.back() : router.replace("/(auth)/phone"))}
      />
    </Screen>
  );
}
