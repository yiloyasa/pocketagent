# PocketAgent phone capture notes

- Captured at: 2026-09-21 (fallback record)
- Device serial attempted: `8e0e7bbe`
- Package: `dev.pocketagent.app`
- Expected screen size: `1080x2400`
- Live capture status: unavailable — `adb devices -l` returned no online devices on 2026-09-21.
- Start evidence: `public/captures/screenshots/phone-demo-start.png`
- End evidence: `public/captures/screenshots/phone-demo-end.png`
- Raw video: not created; the Remotion phone scene uses the clearly labeled `UI SHOWCASE` screenshot fallback.

Verified repository evidence used:

- `docs/verify/evidence/2026-09-16/02_chat_online.png`: PocketAgent connected over USB with Codex selected.
- `docs/verify/evidence/2026-09-15/v19_taskcenter_unread.png`: Task Center showing pending and failed task states.

Commands attempted:

```powershell
adb start-server
adb devices -l
adb -s 8e0e7bbe get-state
adb -s 8e0e7bbe shell pm path dev.pocketagent.app
```

Observed limitation: no live Android device was available to launch or record. The video must not describe a live interaction result as captured; the phone scene is framed as a product UI showcase fallback.
