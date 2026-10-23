# apps/ — mobile apps (React Native, Expo)

Self-contained pnpm workspace: `rider/`, `driver/` and the mobile-shared `packages/` (`api-client` generated from `../backend/api/openapi.yaml`, `ui`, `realtime`, `forms`, `i18n`, `domain`).

```bash
cd apps
pnpm install
pnpm gen:api        # regenerate the typed client after a contract change
pnpm typecheck && pnpm lint
BRAND=nexa pnpm --filter @platform/rider-app start
```
Brand selection: `BRAND=<code>` picks `apps/brands/<code>.json` (build identity only) and `APP_ENV=local|uat|prod` picks the API host; colours, logos and names at runtime come from the backend. To regenerate the client without the backend folder: `OPENAPI_SPEC=https://<api>/v1/openapi.yaml pnpm gen:api`.

## Testing on USB-connected phones
`tools/usb-phones.sh` sets up `adb reverse` for Metro and the backend on each attached phone and opens the app in Expo Go. Start the dev servers with `API_BASE_URL=http://127.0.0.1:8080/v1` so the phones reach the backend through USB regardless of Wi-Fi. Re-run the script whenever ADB restarts (the forwards are lost) — the symptom is Expo Go's "Something went wrong".
