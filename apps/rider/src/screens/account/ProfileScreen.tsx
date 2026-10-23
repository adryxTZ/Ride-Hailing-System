/** ProfileScreen — photo, name, phone, email, identity verification (design: Profile). Name/email edit inline. */
import { Avatar, Button, Header, ListRow, Screen, Section, Text, TextField } from "@platform/ui";
import { useState } from "react";
import { View } from "react-native";
import { useBack } from "@/hooks/useBack";
import { useProfile, useUpdateProfile } from "@/hooks/useProfile";

export function ProfileScreen() {
  const back = useBack();
  const { data: me } = useProfile();
  const update = useUpdateProfile();
  const [editing, setEditing] = useState<"full_name" | "email" | null>(null);
  const [draft, setDraft] = useState("");

  const startEdit = (field: "full_name" | "email") => {
    setDraft(me?.[field] ?? "");
    setEditing(field);
  };
  const save = () => {
    update.mutate({ [editing as string]: draft.trim() } as { full_name?: string; email?: string }, {
      onSuccess: () => setEditing(null),
    });
  };

  return (
    <Screen align="top">
      <Header title="Profile" leading="back" onLeading={() => back()} />
      <View style={{ alignItems: "center", gap: 8 }}>
        <Avatar name={me?.full_name ?? undefined} size={96} editable onPress={() => undefined} />
        <Text variant="muted">Drivers can only see your photo during pickup</Text>
      </View>
      {editing ? (
        <View style={{ gap: 12 }}>
          <TextField
            label={editing === "full_name" ? "Name" : "Email address"}
            value={draft}
            onChangeText={setDraft}
            autoFocus
            keyboardType={editing === "email" ? "email-address" : "default"}
            autoCapitalize={editing === "email" ? "none" : "words"}
            error={update.error ? update.error.message : undefined}
          />
          <Button title="Save" onPress={save} loading={update.isPending} disabled={draft.trim().length < 2} />
          <Button title="Cancel" variant="secondary" onPress={() => setEditing(null)} />
        </View>
      ) : (
        <Section>
          <ListRow
            title="Name"
            subtitle={me?.full_name ?? "Add your name"}
            onPress={() => startEdit("full_name")}
          />
          <ListRow title="Phone number" subtitle={me?.phone} chevron={false} />
          <ListRow
            title="Email address"
            subtitle={me?.email ? `${me.email}${me.email_verified ? " · Verified" : ""}` : "Add an email"}
            onPress={() => startEdit("email")}
          />
          <ListRow
            title="Identity"
            subtitle="Not verified"
            trailing={<Button title="Verify" onPress={() => undefined} />}
          />
        </Section>
      )}
    </Screen>
  );
}
