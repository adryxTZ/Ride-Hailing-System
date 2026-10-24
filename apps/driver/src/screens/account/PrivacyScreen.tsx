/**
 * PrivacyScreen — my data rights: ask for a copy of my data, ask to delete my
 * account (after an inline confirmation), and follow my requests (status, due
 * date, download a ready export, cancel one nobody has picked up).
 */
import {
  DeleteConfirmCard,
  hasOpen,
  PrivacyActionTile,
  type PrivacyKind,
  PrivacyRequestCard,
  sortRequests,
} from "@platform/privacy";
import { Header, Screen, Text } from "@platform/ui";
import { useState } from "react";
import { ActivityIndicator } from "react-native";
import { useBack } from "@/hooks/useBack";
import {
  useCancelPrivacyRequest,
  useCreatePrivacyRequest,
  useDownloadExport,
  usePrivacyRequests,
} from "@/hooks/usePrivacy";

// Entrance order: the two action tiles first, then the requests.
const FIRST_REQUEST_INDEX = 2;

export function PrivacyScreen() {
  const back = useBack();
  const { data, isLoading, error } = usePrivacyRequests();
  const create = useCreatePrivacyRequest();
  const cancel = useCancelPrivacyRequest();
  const download = useDownloadExport();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const requests = sortRequests(data ?? []);
  const ask = (kind: PrivacyKind) => create.mutate(kind, { onSuccess: () => setConfirmingDelete(false) });
  const failure = create.error ?? cancel.error ?? download.error;
  return (
    <Screen align="top">
      <Header title="Privacy" leading="back" onLeading={() => back()} />
      <Text variant="muted">Get a copy of your data or ask us to delete your account.</Text>
      <PrivacyActionTile
        index={0}
        icon="download-outline"
        title="Download my data"
        subtitle="We prepare a file with your account and trips."
        busy={create.isPending && create.variables === "EXPORT"}
        disabled={hasOpen(requests, "EXPORT")}
        onPress={() => ask("EXPORT")}
      />
      <PrivacyActionTile
        index={1}
        icon="account-remove-outline"
        title="Delete my account"
        subtitle="Remove your personal details from the app."
        destructive
        disabled={hasOpen(requests, "DELETE")}
        onPress={() => setConfirmingDelete(true)}
      />
      {confirmingDelete ? (
        <DeleteConfirmCard
          busy={create.isPending && create.variables === "DELETE"}
          onConfirm={() => ask("DELETE")}
          onKeep={() => setConfirmingDelete(false)}
        />
      ) : null}
      {failure ? <Text variant="error">{failure.message}</Text> : null}
      <Text style={{ fontSize: 18, fontWeight: "700" }}>Your requests</Text>
      {isLoading ? <ActivityIndicator /> : null}
      {error ? <Text variant="error">{error.message}</Text> : null}
      {data && requests.length === 0 ? <Text variant="muted">No requests yet.</Text> : null}
      {requests.map((r, i) => (
        <PrivacyRequestCard
          key={r.id}
          index={i + FIRST_REQUEST_INDEX}
          request={r}
          busy={
            download.isPending && download.variables === r.id
              ? "download"
              : cancel.isPending && cancel.variables === r.id
                ? "cancel"
                : undefined
          }
          onDownload={() => download.mutate(r.id)}
          onCancel={() => cancel.mutate(r.id)}
        />
      ))}
    </Screen>
  );
}
