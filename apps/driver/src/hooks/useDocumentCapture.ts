/**
 * useDocumentCapture — "Take photo" or "Choose from gallery" for a document,
 * then (for documents whose expiry the country tracks) its expiry date, then
 * upload: as a sign-up document, or as a renewal for an approved driver.
 * Photos are compressed (the reviewer needs legibility, not megapixels). On the
 * web only the file picker exists. The screen renders `asking` as ExpiryDateSheet.
 */
import * as ImagePicker from "expo-image-picker";
import { useState } from "react";
import { ActionSheetIOS, Alert, Platform } from "react-native";
import { useRenewDocument } from "@/hooks/useDocuments";
import { useUploadDocument } from "@/hooks/useOnboarding";

/** What a capture needs to know about the document. */
export interface CaptureTarget {
  kind: string;
  label: string;
  tracksExpiry: boolean;
  expiryRequired: boolean;
}

/** An open expiry question: the screen shows the sheet and answers through these. */
export interface ExpiryQuestion {
  label: string;
  required: boolean;
  answer: (date?: string) => void;
  cancel: () => void;
}

const PHOTO_QUALITY = 0.6;

type Source = "camera" | "library";

async function pick(source: Source): Promise<ImagePicker.ImagePickerAsset | undefined> {
  const options: ImagePicker.ImagePickerOptions = { mediaTypes: ["images"], quality: PHOTO_QUALITY };
  if (source === "camera") {
    const perm = await ImagePicker.requestCameraPermissionsAsync();
    if (!perm.granted) throw new Error("Allow the camera to photograph your documents.");
    const r = await ImagePicker.launchCameraAsync(options);
    return r.canceled ? undefined : r.assets[0];
  }
  const r = await ImagePicker.launchImageLibraryAsync(options);
  return r.canceled ? undefined : r.assets[0];
}

/** Asks where the photo comes from (camera or gallery). */
function chooseSource(title: string): Promise<Source | undefined> {
  if (Platform.OS === "web") return Promise.resolve("library");
  return new Promise((resolve) => {
    if (Platform.OS === "ios") {
      ActionSheetIOS.showActionSheetWithOptions(
        { title, options: ["Take photo", "Choose from gallery", "Cancel"], cancelButtonIndex: 2 },
        (i) => resolve(i === 0 ? "camera" : i === 1 ? "library" : undefined),
      );
      return;
    }
    Alert.alert(title, undefined, [
      { text: "Take photo", onPress: () => resolve("camera") },
      { text: "Choose from gallery", onPress: () => resolve("library") },
      { text: "Cancel", style: "cancel", onPress: () => resolve(undefined) },
    ]);
  });
}

export function useDocumentCapture(mode: "onboarding" | "renew" = "onboarding") {
  const upload = useUploadDocument();
  const renew = useRenewDocument();
  const [busyKind, setBusyKind] = useState<string>();
  const [error, setError] = useState<string>();
  const [asking, setAsking] = useState<ExpiryQuestion>();

  /** Resolves with the date (or undefined when skipped); rejects when the driver closes the sheet. */
  const askExpiry = (t: CaptureTarget) =>
    new Promise<string | undefined>((resolve, reject) =>
      setAsking({
        label: t.label,
        required: t.expiryRequired,
        answer: (d) => {
          setAsking(undefined);
          resolve(d);
        },
        cancel: () => {
          setAsking(undefined);
          reject(new Error(""));
        },
      }),
    );

  const capture = async (t: CaptureTarget) => {
    setError(undefined);
    try {
      const source = await chooseSource(t.label);
      if (!source) return;
      const asset = await pick(source);
      if (!asset) return;
      const expiresOn = t.tracksExpiry ? await askExpiry(t) : undefined;
      setBusyKind(t.kind);
      const type = asset.mimeType === "image/png" ? "image/png" : "image/jpeg";
      const send = mode === "renew" ? renew : upload;
      await send.mutateAsync({ kind: t.kind, uri: asset.uri, type, expiresOn });
    } catch (e) {
      setError((e as Error).message || undefined);
    } finally {
      setBusyKind(undefined);
    }
  };
  return { capture, busyKind, error, asking };
}
