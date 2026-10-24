// Accepts the current privacy notice for test accounts, so scripts that book or
// go online pass the consent gate. Run after a new notice version:
//   node tools/accept-consent.mjs [phone:app ...]   (default: the rider and driver test numbers)
const api = process.env.API ?? "http://localhost:8080/v1";
const accounts = process.argv.slice(2).length ? process.argv.slice(2) : ["+255626512332:rider", "+255715123702:driver"];
const post = async (path, body, token) => {
  const r = await fetch(api + path, { method: "POST", headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: JSON.stringify(body) });
  const text = await r.text();
  if (!r.ok) throw new Error(`${path} → ${r.status} ${text.slice(0, 200)}`);
  return text ? JSON.parse(text) : null;
};
for (const a of accounts) {
  const [phone, app] = a.split(":");
  await post("/auth/otp/request", { phone, app });
  const t = await post("/auth/otp/verify", { phone, app, code: "0000" });
  if (!t.consent_required) { console.log(`${phone} ${app}: already accepted`); continue; }
  const doc = await (await fetch(`${api}/legal/consent?app=${app}`)).json();
  await post("/me/consents", { app: app.toUpperCase(), version: doc.version }, t.access_token);
  console.log(`${phone} ${app}: accepted v${doc.version}`);
}
